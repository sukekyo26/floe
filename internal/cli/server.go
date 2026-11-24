package cli

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"floe/internal/daemon"
	"floe/internal/drawproc"
	"floe/internal/files"
	"floe/internal/link"
	"floe/internal/server"
	"floe/internal/store"
	"floe/internal/usererr"
)

const defaultPort = 7325

// decimalRe is a port written in decimal digits alone: hex (0x10), a sign and exponents (1e3) are typos.
var decimalRe = regexp.MustCompile(`^[0-9]+$`)

// settingKeys are the environment variables the server's settings come from: a server a command starts takes
// those the command read, so that it means the same directories.
var settingKeys = []string{
	link.HomeKey, dataBase.key, configBase.key, "FLOE_DIR", "FLOE_PORT", "FLOE_PARTS_DIR", "FLOE_THEMES_DIR",
	"FLOE_TEMPLATES_DIR", "FLOE_ICONS_DIR",
}

// storeDir is the directory of the pages: FLOE_DIR, else storeSub under the base of the data (dataDir).
func (a *app) storeDir() (dataPlace, error) {
	p, err := a.dataDir("FLOE_DIR", dataBase, storeSub)
	switch {
	case err != nil:
		return p, err
	case p.unusable != nil:
		return p, p.unusable // the server writes there
	case p.dir == "":
		return p, usererr.New("store_no_dir", p.from)
	}
	return p, nil
}

// port is the server's port, FLOE_PORT else 7325.
func (a *app) port() (int, error) {
	raw := a.getenv("FLOE_PORT")
	if raw == "" {
		return defaultPort, nil
	}
	port, err := strconv.Atoi(raw)
	if !decimalRe.MatchString(raw) || err != nil || port < 1 || port > 65535 {
		return 0, usererr.New("cli_invalid_port", raw)
	}
	return port, nil
}

// serverConfig is where the server serves and what it draws with, as this command reads the settings, and the
// place of the store, which serve writes in: the server a command finds must mean the same. The icons are none
// ("") when there is no place for them, as a command draws then (iconStore); any other mistake of a setting stops
// it.
func (a *app) serverConfig() (server.Config, dataPlace, error) {
	var cfg server.Config
	var found usererr.List
	root, err := a.storeDir()
	if failed := found.Add(err); failed != nil {
		return cfg, root, failed
	}
	port, err := a.port()
	if failed := found.Add(err); failed != nil {
		return cfg, root, failed
	}
	for _, d := range []struct {
		kind string
		to   *string
	}{{"parts", &cfg.Parts}, {"themes", &cfg.Themes}, {"templates", &cfg.Templates}} {
		p, placeErr := a.ownPlace(d.kind)
		if failed := found.Add(placeErr); failed != nil {
			return cfg, root, failed
		}
		*d.to = p.toRead()
	}
	icons, err := a.iconsDir()
	if e, ok := errors.AsType[*usererr.Error](err); ok && e.Key == "icons_no_dir" {
		err = nil
	}
	if failed := found.Add(err); failed != nil {
		return cfg, root, failed
	}
	if err := found.Err(); err != nil {
		return cfg, root, err //nolint:wrapcheck // mistakes for the user as they are
	}
	cfg.Store, cfg.Port, cfg.Icons, cfg.Home = store.New(root.dir), port, icons.toRead(), a.getenv(link.HomeKey)
	cfg.Drawing = a.drawing
	return cfg, root, nil
}

// errNoDrawer is the failure of a server that has not set what it draws with (serve sets it).
var errNoDrawer = errors.New("the server has no floe __draw to draw with")

// drawing is what the server draws pages with: one's own over floe's (kit) and the icons (iconStore), which floe
// __draw reads as the server does and draws with (a.drawer, which serve sets).
func (a *app) drawing() (store.Drawing, error) {
	if a.drawer == nil {
		return store.Drawing{Kit: nil, Icons: nil, Draw: nil, Err: nil}, errNoDrawer
	}
	k, err := a.kit(true)
	if err != nil {
		return store.Drawing{Kit: nil, Icons: nil, Draw: nil, Err: nil}, err
	}
	return store.Drawing{Kit: k, Icons: a.iconStore(), Draw: a.drawer, Err: nil}, nil
}

// drawTime is how long the server lets one page be drawn: drawproc.Limit, shorter in a test.
var drawTime = drawproc.Limit

// drawChildren is the Drawer of the server of build: floe __draw, the binary exe, with the settings the server has.
// LIMIT: floe __draw reads one's own parts, themes and templates itself, as the server does each second: one changed
// in between is drawn with as it is now, a second's difference, as the server's own drawing had.
func (a *app) drawChildren(build, exe string) *drawproc.Child {
	env := a.serverEnv()
	return drawproc.NewChild(build, drawTime, func(ctx context.Context) *exec.Cmd {
		cmd := exec.CommandContext(ctx, exe, "__draw")
		cmd.Env = env
		return cmd
	})
}

// client makes sure the server of this store runs, of this build, and returns what calls it. anyDirs
// takes a server that draws with other directories than this command's, for one that draws nothing.
func (a *app) client(ctx context.Context, anyDirs bool) (client, error) {
	cfg, _, err := a.serverConfig()
	if err != nil {
		return client{}, err
	}
	build, err := daemon.Build()
	if err != nil {
		return client{}, err //nolint:wrapcheck // already says what failed
	}
	stopped, err := daemon.Ensure(ctx, daemon.Server{
		Port: cfg.Port, Root: cfg.Store.Root(), Build: build, Templates: cfg.Templates, Parts: cfg.Parts,
		Themes: cfg.Themes, Icons: cfg.Icons, AnyDirs: anyDirs, Env: a.serverEnv(),
	})
	// Told before the mistake, so that a server stopped and not started in its place is not lost.
	if stopped != 0 {
		a.warn("cli_replaced_server", cfg.Port, stopped)
	}
	if err != nil {
		return client{}, err //nolint:wrapcheck // the message is for the user as is
	}
	return client{base: daemon.Base(cfg.Port)}, nil
}

// serverEnv is the environment of a server that a command starts: the process's, with the settings this command
// read over it (one it reads as unset, unset there too).
func (a *app) serverEnv() []string {
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool {
		key, _, _ := strings.Cut(kv, "=")
		return slices.Contains(settingKeys, key)
	})
	for _, key := range settingKeys {
		if v := a.getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	return env
}

func (a *app) serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the server in the foreground",
		Args:  usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.serve(cmd.Context())
		},
	}
}

// serve runs until SIGTERM or SIGINT, or "floe stop" asks the server to stop. A second signal, while the requests
// under way are finished, ends the process at once.
func (a *app) serve(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	context.AfterFunc(ctx, stop)
	cfg, at, err := a.serverConfig()
	if err != nil {
		return err
	}
	root := cfg.Store.Root()
	if err = writable(root); err != nil {
		return usererr.SettingDir(at.setting(), root, err, usererr.CannotWrite(root, err))
	}
	if cfg.Build, err = daemon.Build(); err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find the floe binary: %w", err)
	}
	a.drawer = a.drawChildren(cfg.Build, exe)
	ln, err := a.listen(cfg.Port)
	if err != nil {
		return err
	}
	// With the port taken: a floe that cannot serve writes nothing in the store. Told before it says it serves.
	if err = cfg.Store.Begin(server.Viewer, store.Now()); err != nil {
		_ = ln.Close() //nolint:errcheck // nothing served
		return err     //nolint:wrapcheck // a usererr, at the file
	}
	// The log is the server's alone to write, and lives as long as the process: it is not closed. The runtime keeps
	// its own copy of the descriptor for what brings the process down.
	logFile, err := server.OpenLog(root)
	if err != nil {
		_ = ln.Close() //nolint:errcheck // nothing served
		return err     //nolint:wrapcheck // a usererr, at the file
	}
	if err = debug.SetCrashOutput(logFile, debug.CrashOptions{}); err != nil {
		_ = ln.Close() //nolint:errcheck // nothing served
		return fmt.Errorf("write a crash to %s: %w", server.LogName, err)
	}
	cfg.Log = logFile
	srv := server.New(cfg)
	a.print("%s", english.Msg("cli_serving", root, daemon.Base(cfg.Port)))
	return srv.Serve(ctx, ln) //nolint:wrapcheck // already says what failed
}

// writable reports why dir, made when missing, cannot be written in: by making a file in it.
func writable(dir string) error {
	if err := files.MkdirAll(dir, 0o700); err != nil {
		return err //nolint:wrapcheck // told at the store
	}
	root, err := files.OpenRoot(dir)
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	defer root.Close() //nolint:errcheck // only looked in
	name := ".check-" + rand.Text()
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	_ = f.Close()            //nolint:errcheck // nothing written
	return root.Remove(name) //nolint:wrapcheck // as above
}

// listen takes the listener the command that started this server bound for it, or binds the port.
func (a *app) listen(port int) (net.Listener, error) {
	if ln, err := daemon.Handed(a.getenv, port); ln != nil || err != nil {
		// Taken once: a floe run from here binds its own.
		_ = os.Unsetenv(daemon.ListenerEnv) //nolint:errcheck // a name Unsetenv takes
		return ln, err                      //nolint:wrapcheck // already says what failed
	}
	ln, err := daemon.Listen(port)
	if err != nil {
		return nil, daemon.CannotListen(port, err) //nolint:wrapcheck // the user's mistake or floe's failure
	}
	return ln, nil
}

func (a *app) stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background server",
		Args:  usage(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			port, err := a.port()
			if err != nil {
				return err
			}
			// LIMIT: a server still starting holds the port but does not answer yet, and is told not running; it comes
			// up after. Wait while the port is bound if that comes to matter.
			p := daemon.Probe(cmd.Context(), daemon.Base(port))
			if p.State != daemon.Ours {
				a.print("%s", english.Msg("cli_not_running"))
				return nil
			}
			q, _, err := daemon.Stop(cmd.Context(), port, p)
			if err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			if q.State == daemon.Ours {
				a.print("%s", english.Msg("cli_stopped_other", p.Health.PID, q.Health.PID))
				return nil
			}
			a.print("%s", english.Msg("cli_stopped", p.Health.PID))
			return nil
		},
	}
}
