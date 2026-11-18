package installsh_test

// A probe for S405, not to be merged: what Invoke-WebRequest does at -TimeoutSec, against servers that stop.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const probeScript = `
$ProgressPreference = 'SilentlyContinue'
$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSEdition -eq 'Desktop') {
  Add-Type -TypeDefinition 'using System.Net; using System.Security.Cryptography.X509Certificates; public class ProbeTrust : ICertificatePolicy { public bool CheckValidationResult(ServicePoint p, X509Certificate c, WebRequest r, int e) { return true; } }'
  [Net.ServicePointManager]::CertificatePolicy = New-Object ProbeTrust
}
$named = @{ UseBasicParsing = $true; Uri = $env:PROBE_URL; MaximumRedirection = 0; Proxy = $env:PROBE_PROXY }
if ($env:PROBE_TIMEOUT) { $named.TimeoutSec = [int]$env:PROBE_TIMEOUT }
$has = (Get-Command Microsoft.PowerShell.Utility\Invoke-WebRequest).Parameters.ContainsKey('OperationTimeoutSeconds')
if ($has -and $env:PROBE_OP) { $named.OperationTimeoutSeconds = [int]$env:PROBE_OP }
if ($PSVersionTable.PSEdition -ne 'Desktop') { $named.SkipCertificateCheck = $true }
$sw = [Diagnostics.Stopwatch]::StartNew()
$problem = $null; $got = $null
try {
  $got = Invoke-WebRequest @named -ErrorAction SilentlyContinue -ErrorVariable problem
  if ($null -eq $got) { throw $problem[0] }
  $n = $got.RawContentStream.ToArray().Length
  "OK   ps=$($PSVersionTable.PSVersion) hasOp=$has t=$([math]::Round($sw.Elapsed.TotalSeconds,1)) status=$([int]$got.StatusCode) bytes=$n"
} catch {
  $e = $_.Exception
  $inner = ''
  $i = $e.InnerException
  while ($i) { $inner += $i.GetType().FullName + ' / '; $i = $i.InnerException }
  "ERR  ps=$($PSVersionTable.PSVersion) hasOp=$has t=$([math]::Round($sw.Elapsed.TotalSeconds,1)) type=$($e.GetType().FullName) status=$($e.Status) response=$([bool]$e.Response) fqid=$($_.FullyQualifiedErrorId) inner=[$inner] msg=[$($e.Message)]"
}
`

// probeSite is a TLS server that stops in the ways asked, behind a proxy: "tunnel" tunnels every CONNECT to it,
// "mute" answers 200 and goes no further, "deaf" never answers the CONNECT.
func probeSite(t *testing.T, proxyKind string) string {
	t.Helper()
	block := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/nohead", func(http.ResponseWriter, *http.Request) { <-block })
	mux.HandleFunc("/stall", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100000")
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1000))
		w.(http.Flusher).Flush() //nolint:forcetypeassert // a probe
		<-block
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "8000")
		for range 8 {
			_, _ = w.Write(bytes.Repeat([]byte("x"), 1000))
			w.(http.Flusher).Flush() //nolint:forcetypeassert // a probe
			time.Sleep(time.Second)
		}
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	tlsServer := httptest.NewTLSServer(mux)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusBadRequest)
			return
		}
		if proxyKind == "deaf" {
			<-block
			return
		}
		conn, rw, err := w.(http.Hijacker).Hijack() //nolint:forcetypeassert // a probe
		if err != nil {
			return
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n")
		if proxyKind == "mute" {
			<-block
			_ = conn.Close()
			return
		}
		up, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", tlsServer.Listener.Addr().String())
		if err != nil {
			_ = conn.Close()
			return
		}
		sent := io.MultiReader(io.LimitReader(rw.Reader, int64(rw.Reader.Buffered())), conn)
		go func() { _, _ = io.Copy(up, sent); _ = up.Close() }()
		_, _ = io.Copy(conn, up)
		_ = conn.Close()
	}))
	t.Cleanup(func() {
		close(block)
		proxy.CloseClientConnections()
		tlsServer.CloseClientConnections()
		proxy.Close()
		tlsServer.Close()
	})
	return proxy.URL
}

func TestProbeTimeout(t *testing.T) {
	t.Parallel()
	script := filepath.Join(t.TempDir(), "probe.ps1")
	if err := os.WriteFile(script, []byte(probeScript), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, shell := range []string{"powershell", "pwsh"} {
		for _, c := range []struct {
			name, proxy, path, timeout, op string
			limit                          time.Duration
		}{
			{"ok", "tunnel", "/ok", "3", "3", time.Minute},
			{"no headers", "tunnel", "/nohead", "3", "3", 2 * time.Minute},
			{"no headers, timeout 20", "tunnel", "/nohead", "20", "20", 2 * time.Minute},
			{"mute after 200", "mute", "/ok", "3", "3", 2 * time.Minute},
			{"deaf to CONNECT", "deaf", "/ok", "3", "3", 2 * time.Minute},
			{"body stalls", "tunnel", "/stall", "3", "3", 7 * time.Minute},
			{"body stalls, no op timeout", "tunnel", "/stall", "3", "", 90 * time.Second},
			{"slow 8s, moving", "tunnel", "/slow", "3", "3", 2 * time.Minute},
			{"no headers, no timeout", "tunnel", "/nohead", "", "", 45 * time.Second},
			{"ok, timeout 86400", "tunnel", "/ok", "86400", "86400", time.Minute},
			{"ok, timeout 2147483", "tunnel", "/ok", "2147483", "2147483", time.Minute},
			{"ok, timeout 2147484", "tunnel", "/ok", "2147484", "2147484", time.Minute},
			{"ok, timeout 99999999", "tunnel", "/ok", "99999999", "99999999", time.Minute},
			{"ok, timeout 2147483647", "tunnel", "/ok", "2147483647", "2147483647", time.Minute},
		} {
			t.Run(shell+"/"+c.name, func(t *testing.T) {
				t.Parallel()
				proxy := probeSite(t, c.proxy)
				var out bytes.Buffer
				start := time.Now()
				err := runLimited(t.Context(), c.limit, func(ctx context.Context) *exec.Cmd {
					cmd := exec.CommandContext(ctx, shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script) //nolint:gosec // a probe
					cmd.Env = append(os.Environ(), "PROBE_URL=https://probe.test"+c.path, "PROBE_PROXY="+proxy,
						"PROBE_TIMEOUT="+c.timeout, "PROBE_OP="+c.op)
					cmd.Stdout = &out
					cmd.Stderr = &out
					return cmd
				}, func() {})
				took := strconv.FormatFloat(time.Since(start).Seconds(), 'f', 1, 64)
				switch {
				case errors.Is(err, errOverLimit):
					t.Errorf("PROBE %s | %s | timeout=%q op=%q | STILL RUNNING at %v, killed (process %ss) | %s", shell, c.name, c.timeout, c.op, c.limit, took, out.String())
				default:
					t.Errorf("PROBE %s | %s | timeout=%q op=%q | process %ss err=%v | %s", shell, c.name, c.timeout, c.op, took, err, out.String())
				}
			})
		}
	}
	_ = fmt.Sprint
}
