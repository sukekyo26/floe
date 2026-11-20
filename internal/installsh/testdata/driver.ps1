# The driver of one run of install.ps1 in TestInstallPS1, dot-sourced by `powershell -Command`. The case is the JSON in
# $env:FLOE_TEST_CASE. It sends the real Invoke-WebRequest through the test's proxy, sends HKCU to a key of its own so
# that the user's PATH of whoever runs the test is never written, runs install.ps1, and writes what it saw as JSON.
$floeTest = $env:FLOE_TEST_CASE | ConvertFrom-Json

# The test's server has a certificate of its own, trusted here alone.
if ($PSVersionTable.PSEdition -eq 'Desktop') {
  Add-Type -TypeDefinition 'using System.Net; using System.Security.Cryptography.X509Certificates; public class FloeTestTrust : ICertificatePolicy { public bool CheckValidationResult(ServicePoint p, X509Certificate c, WebRequest r, int e) { return true; } }'
  [Net.ServicePointManager]::CertificatePolicy = New-Object FloeTestTrust
}
# Refuses a call without what install.ps1 must give it: Windows PowerShell asks before it parses a page without
# -UseBasicParsing, which nothing answers in a script.
function global:Invoke-WebRequest {
  # The arguments by name: $args passes "-Name:" and its value on as two, which the cmdlet then reads apart.
  $named = @{}
  for ($i = 0; $i -lt $args.Count; $i++) {
    $name = ([string]$args[$i]).TrimStart('-').TrimEnd(':')
    $next = $i + 1 -lt $args.Count -and -not ($args[$i + 1] -is [string] -and $args[$i + 1].StartsWith('-'))
    if ($next) { $named[$name] = $args[$i + 1]; $i++ } else { $named[$name] = $true }
  }
  foreach ($need in 'UseBasicParsing', 'MaximumRedirection') {
    if (-not $named.ContainsKey($need)) { throw "the test's Invoke-WebRequest: called without -$need" }
  }
  if ($named.MaximumRedirection -ne 0) { throw "the test's Invoke-WebRequest: -MaximumRedirection $($named.MaximumRedirection), not 0" }
  $named.Proxy = $global:floeTest.proxy
  if ($PSVersionTable.PSEdition -ne 'Desktop') { $named.SkipCertificateCheck = $true }
  # Windows PowerShell 5.1 now and then sends nothing on a tunnel the proxy opened, and the request times out without
  # having reached the server (.NET Framework's own): such a request is sent once more, once in a run. See runPS.
  # LIMIT: a request that reached the server on one tunnel and timed out, while another tunnel .NET opened meanwhile
  # stayed silent, would be sent again too. Only the case "stop: no tunnel left silent" times out on psWait with the
  # server reached; a stop of Windows PowerShell's on a tunnel of its own in that minute fails it (Retried 1).
  for (;;) {
    $last = if ($Error.Count) { $Error[0] } else { $null }
    # The count is read only where a request may be sent again: PowerShell 7's time-out never is.
    $may = $global:floeTest.retryStop -and $global:floeRetried -eq 0 -and $PSVersionTable.PSEdition -eq 'Desktop'
    if ($may) { $silent = FloeSilent }
    $got = $null; $failed = $null; $thrown = $false
    try {
      $got = Microsoft.PowerShell.Utility\Invoke-WebRequest @named
      if ($Error.Count -and -not [object]::ReferenceEquals($Error[0], $last)) { $failed = $Error[0] }
    } catch { $failed = $_; $thrown = $true }
    # Windows PowerShell's time-out; PowerShell 7's is no WebException and is never sent again.
    $timedOut = $null -ne $failed -and $failed.Exception -is [Net.WebException] -and $failed.Exception.Status -eq [Net.WebExceptionStatus]::Timeout
    if ($may -and $null -eq $got -and $timedOut -and $null -ne $silent) {
      $now = FloeSilent
      if ($null -ne $now -and $now -gt $silent) {
        $global:floeRetried++
        continue
      }
    }
    if ($thrown) { throw $failed }
    return $got
  }
}
$global:floeRetried = 0
$global:floeSilentFailed = ''
# The count of tunnels the test's proxy opened and heard nothing on (site in the test), or $null when it cannot be
# read: then nothing is sent again, and the report tells why.
function global:FloeSilent {
  try {
    $ask = [Net.WebRequest]::Create($global:floeTest.proxy + '/floe-test/silent')
    $ask.Proxy = $null
    $ask.Timeout = 10000
    $answer = $ask.GetResponse()
    try { [int](New-Object IO.StreamReader($answer.GetResponseStream())).ReadToEnd() } finally { $answer.Close() }
  } catch {
    $global:floeSilentFailed = "$_"
    $null
  }
}
if ($floeTest.failMove) {
  # The rename that puts the new file in place fails; the one that moves floe.exe aside, and the one back, do not.
  function global:Move-Item([string]$LiteralPath, [string]$Destination) {
    if ((Split-Path -Leaf $LiteralPath) -like 'floe.exe.new-*') { throw "the test's Move-Item: refused $LiteralPath" }
    Microsoft.PowerShell.Management\Move-Item -LiteralPath $LiteralPath -Destination $Destination
  }
}
if ($floeTest.failCopy) {
  # The copy to the install folder is cut short: part of the file is written.
  function global:Copy-Item([string]$LiteralPath, [string]$Destination) {
    [IO.File]::WriteAllText($Destination, 'part')
    throw "the test's Copy-Item: cut short at $Destination"
  }
}
if ($floeTest.failCleanup) {
  # The temporary folder cannot be removed, as when a scanner holds a file in it: an error that does not stop.
  function global:Remove-Item {
    [CmdletBinding()]
    param([string]$LiteralPath, [switch]$Recurse, [switch]$Force)
    if ((Split-Path -Leaf $LiteralPath) -like 'floe-install-*') { Write-Error "the test's Remove-Item: held $LiteralPath"; return }
    Microsoft.PowerShell.Management\Remove-Item -LiteralPath $LiteralPath -Recurse:$Recurse -Force:$Force
  }
}

Add-Type -TypeDefinition @'
using System; using System.Runtime.InteropServices;
public static class FloeTestRegistry {
  [DllImport("advapi32.dll")] public static extern int RegOverridePredefKey(IntPtr key, IntPtr to);
  public static readonly IntPtr CurrentUser = new IntPtr(unchecked((int)0x80000001));
}
'@
$floeSoftware = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Software', $true)
$floeNow = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
# A key left by a run that was killed: its name holds the time it was made.
foreach ($n in $floeSoftware.GetSubKeyNames()) {
  if ($n -match '^floe-install-test-(\d+)-' -and [long]$Matches[1] -lt $floeNow - 3600) { $floeSoftware.DeleteSubKeyTree($n, $false) }
}
$floeKeyName = "floe-install-test-$floeNow-" + [IO.Path]::GetRandomFileName()
$floeKey = $floeSoftware.CreateSubKey($floeKeyName)
$floeEnv = $floeKey.CreateSubKey('Environment')
if ($floeTest.pathKind -cne 'none') {
  $floeValue = $floeTest.path
  if ($floeTest.pathKind -ceq 'MultiString') { $floeValue = [string[]]($floeTest.path -split '\|') }
  $floeEnv.SetValue($floeTest.pathName, $floeValue, [Microsoft.Win32.RegistryValueKind]$floeTest.pathKind)
}
if ($floeTest.notice) { $floeEnv.SetValue('FLOE_INSTALL_PATH_NOTICE', 'kept') }
$floeMark = [IO.Path]::GetRandomFileName()
$floeEnv.SetValue('FloeTestMark', $floeMark)
$floeEnv.Close()
$floeRc = [FloeTestRegistry]::RegOverridePredefKey([FloeTestRegistry]::CurrentUser, $floeKey.Handle.DangerousGetHandle())
$floeCheck = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
if ($floeRc -ne 0 -or -not $floeCheck -or $floeCheck.GetValue('FloeTestMark') -cne $floeMark) {
  [void][FloeTestRegistry]::RegOverridePredefKey([FloeTestRegistry]::CurrentUser, [IntPtr]::Zero)
  $floeSoftware.DeleteSubKeyTree($floeKeyName, $false)
  throw "the test could not send HKCU to a key of its own (RegOverridePredefKey: $floeRc); install.ps1 is not run"
}
$floeCheck.Close()

$floeReport = [ordered]@{}
try {
  # Set here, not for the process: Add-Type above compiles in TEMP, and fails in some folders install.ps1 must take.
  if ($floeTest.tmp) { $env:TMP = $floeTest.tmp; $env:TEMP = $floeTest.tmp }
  if ($floeTest.cd) { Set-Location -LiteralPath $floeTest.cd }
  if ($floeTest.strict) { Set-StrictMode -Version Latest }
  if ($floeTest.run) { Invoke-Expression $floeTest.run }
  elseif ($floeTest.form -ceq 'file') { & $floeTest.script }
  else { Get-Content -Raw -LiteralPath $floeTest.script | Invoke-Expression }
  $floeReport.eap = "$ErrorActionPreference"
  $floeReport.progress = "$ProgressPreference"
} finally {
  # Whether the caller is still strict, after a failure too.
  try { $floeObject = [pscustomobject]@{ a = 1 }; $null = $floeObject.nope; $floeReport.strict = $false } catch { $floeReport.strict = $true }
  Set-StrictMode -Off
  Set-Location -LiteralPath $floeTest.home
  $floeEnv = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
  $floeValue = $floeEnv.GetValue('Path', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
  if ($null -eq $floeValue) { $floeReport.pathKind = 'none'; $floeReport.path = '' }
  else { $floeReport.pathKind = '' + $floeEnv.GetValueKind('Path'); $floeReport.path = (@($floeValue) -join '|') }
  $floeReport.pathName = '' + ($floeEnv.GetValueNames() | Where-Object { $_ -ieq 'path' })
  $floeReport.notice = $null -ne $floeEnv.GetValue('FLOE_INSTALL_PATH_NOTICE')
  $floeReport.retried = $global:floeRetried
  $floeReport.silentFailed = $global:floeSilentFailed
  $floeEnv.Close()
  [void][FloeTestRegistry]::RegOverridePredefKey([FloeTestRegistry]::CurrentUser, [IntPtr]::Zero)
  $floeSoftware.DeleteSubKeyTree($floeKeyName, $false)
  [IO.File]::WriteAllText($floeTest.report, ($floeReport | ConvertTo-Json -Compress))
}
