package cli

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/sukekyo26/floe/internal/usererr"
)

const defaultPort = 7325

var (
	decimalIntRe = regexp.MustCompile(`^[0-9]+$`)
	decimalRe    = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)$`)
)

// config is what floe reads from the environment. Every command checks it first.
type config struct {
	root string
	home string
	port int
	idle time.Duration
	// raw values, handed on to a server that a publish starts
	rawPort, rawIdle string
}

func (c *config) base() string {
	return "http://127.0.0.1:" + strconv.Itoa(c.port)
}

func loadConfig(getenv func(string) string) (*config, error) {
	c := &config{root: "", home: getenv("HOME"), port: defaultPort, idle: 30 * time.Minute, rawPort: "", rawIdle: ""}
	c.rawPort, c.rawIdle = getenv("FLOE_PORT"), getenv("FLOE_IDLE_MINUTES")
	if c.rawPort != "" {
		// Decimal only: hex (0x10) and exponents (1e3) are typos, not ports.
		port, err := strconv.Atoi(c.rawPort)
		if !decimalIntRe.MatchString(c.rawPort) || err != nil || port < 1 || port > 65535 {
			return nil, usererr.New(fmt.Sprintf(`invalid FLOE_PORT "%s": use a port number from 1 to 65535`, c.rawPort), "")
		}
		c.port = port
	}
	if c.rawIdle != "" {
		minutes, err := strconv.ParseFloat(c.rawIdle, 64)
		if !decimalRe.MatchString(c.rawIdle) || err != nil || minutes <= 0 {
			return nil, usererr.New(
				fmt.Sprintf(`invalid FLOE_IDLE_MINUTES "%s": use a positive number of minutes`, c.rawIdle), "")
		}
		// Capped where the JS version's setTimeout was (2^31-1 ms, about 24.8 days).
		c.idle = time.Duration(math.Min(minutes*60_000, math.MaxInt32)) * time.Millisecond
	}
	c.root = getenv("FLOE_DIR")
	if c.root == "" {
		data := getenv("XDG_DATA_HOME")
		if data == "" {
			if c.home == "" {
				return nil, usererr.New("HOME is not set; set HOME or FLOE_DIR", "")
			}
			data = filepath.Join(c.home, ".local", "share")
		}
		c.root = filepath.Join(data, "floe")
	}
	return c, nil
}
