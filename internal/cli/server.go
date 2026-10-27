package cli

import (
	"context"
	"errors"
	"net"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sukekyo26/floe/internal/daemon"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

const defaultPort = 7325

// decimalRe is a port written in decimal digits alone: hex (0x10), a sign and exponents (1e3) are typos.
var decimalRe = regexp.MustCompile(`^[0-9]+$`)

// settingKeys are the environment variables the server's settings come from: a server a command starts takes
// those the command read, so that it means the same directories.
var settingKeys = []string{
	"HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "FLOE_DIR", "FLOE_PORT", "FLOE_PARTS_DIR", "FLOE_THEMES_DIR",
	"FLOE_TEMPLATES_DIR", "FLOE_ICONS_DIR",
}

// storeDir is the directory of the pages: FLOE_DIR, else floe under the XDG data directory, else under
// ~/.local/share (dataDir).
func (a *app) storeDir() (string, error) {
	dir, _, err := a.dataDir("FLOE_DIR", "XDG_DATA_HOME", ".local/share", "floe")
	switch {
	case err != nil:
		return "", err
	case dir == "":
		return "", usererr.New("store_no_dir")
	}
	return dir, nil
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

// serverConfig is where the server serves and what it draws with, as this command reads the settings: the server
// a command finds must mean the same. The icons are none ("") when there is no place for them, as a command draws
// then (iconStore); any other mistake of a setting stops it.
func (a *app) serverConfig() (server.Config, error) {
	var cfg server.Config
	var found usererr.List
	root, err := a.storeDir()
	if failed := found.Add(err); failed != nil {
		return cfg, failed
	}
	port, err := a.port()
	if failed := found.Add(err); failed != nil {
		return cfg, failed
	}
	for _, d := range []struct {
		kind string
		to   *string
	}{{"parts", &cfg.Parts}, {"themes", &cfg.Themes}, {"templates", &cfg.Templates}} {
		dir, _, _, placeErr := a.ownPlace(d.kind)
		if failed := found.Add(placeErr); failed != nil {
			return cfg, failed
		}
		*d.to = dir
	}
	icons, err := a.iconsDir()
	if e, ok := errors.AsType[*usererr.Error](err); ok && e.Key == "icons_no_dir" {
		err = nil
	}
	if failed := found.Add(err); failed != nil {
		return cfg, failed
	}
	if err := found.Err(); err != nil {
		return cfg, err //nolint:wrapcheck // mistakes for the user as they are
	}
	cfg.Store, cfg.Port, cfg.Icons, cfg.Home = store.New(root), port, icons, a.getenv("HOME")
	cfg.Drawing = a.drawing
	return cfg, nil
}

// drawing is what the server draws pages with: one's own over floe's (kit) and the icons (iconStore).
func (a *app) drawing() (store.Drawing, error) {
	k, err := a.kit(true)
	if err != nil {
		return store.Drawing{Kit: nil, Icons: nil, Err: nil}, err
	}
	return store.Drawing{Kit: k, Icons: a.iconStore(), Err: nil}, nil
}

// base is the address of the server of cfg.
func base(cfg server.Config) string {
	return "http://" + daemon.Addr(cfg.Port)
}

// client makes sure the server of this store runs, of this build, and returns what calls it. anyDirs
// takes a server that draws with other directories than this command's, for one that draws nothing.
func (a *app) client(ctx context.Context, anyDirs bool) (client, error) {
	cfg, err := a.serverConfig()
	if err != nil {
		return client{}, err
	}
	build, err := daemon.Build()
	if err != nil {
		return client{}, err //nolint:wrapcheck // already says what failed
	}
	err = daemon.Ensure(ctx, daemon.Server{
		Port: cfg.Port, Root: cfg.Store.Root(), Build: build, Templates: cfg.Templates, Parts: cfg.Parts,
		Themes: cfg.Themes, Icons: cfg.Icons, AnyDirs: anyDirs, Env: a.serverEnv(),
	})
	if err != nil {
		return client{}, err //nolint:wrapcheck // the message is for the user as is
	}
	return client{base: base(cfg)}, nil
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

// serve runs until SIGTERM or SIGINT ("floe stop" sends SIGTERM).
func (a *app) serve(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	cfg, err := a.serverConfig()
	if err != nil {
		return err
	}
	root := cfg.Store.Root()
	if err = writable(root); err != nil {
		return usererr.CannotWrite(root, err) //nolint:wrapcheck // the system's words, at the store
	}
	if cfg.Build, err = daemon.Build(); err != nil {
		return err //nolint:wrapcheck // already says what failed
	}
	ln, err := a.listen(cfg.Port)
	if err != nil {
		return err
	}
	srv := server.New(cfg)
	a.print("%s", english.Msg("cli_serving", root, base(cfg)))
	return srv.Serve(ctx, ln) //nolint:wrapcheck // already says what failed
}

// writable reports why dir, made when missing, cannot be written in: by making a file in it.
func writable(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err //nolint:wrapcheck // told at the store
	}
	f, err := os.CreateTemp(dir, ".check-")
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	_ = f.Close()              //nolint:errcheck // nothing written
	return os.Remove(f.Name()) //nolint:wrapcheck // as above
}

// listen takes the listener the publish that started this server bound for it, or binds the port.
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
			p := daemon.Probe(cmd.Context(), "http://"+daemon.Addr(port))
			if p.State != daemon.Ours {
				a.print("%s", english.Msg("cli_not_running"))
				return nil
			}
			if err := daemon.Terminate(p.Health.PID); err != nil {
				return err //nolint:wrapcheck // already says what failed
			}
			a.print("%s", english.Msg("cli_stopped", p.Health.PID))
			return nil
		},
	}
}
