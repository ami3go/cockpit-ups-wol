package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/version"
)

func main() {
	fs := flag.NewFlagSet("cockpit-ups-wol-agent", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showVersion := fs.Bool("version", false, "print version information and exit")
	configPath := fs.String("config", "/etc/cockpit-ups-wol/config.yaml", "configuration file")
	_ = fs.Bool("foreground", false, "run in foreground (default for systemd)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: cockpit-ups-wol-agent [options]\n\n")
		fmt.Fprintf(fs.Output(), "Power-management agent for cockpit-ups-wol.\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showVersion {
		b, _ := json.Marshal(version.Current())
		fmt.Println(string(b))
		return
	}

	// Runtime implementation is added in subsequent v0.1 tasks. The foundation
	// deliberately exits with a clear error instead of pretending protection is active.
	fmt.Fprintf(os.Stderr, "cockpit-ups-wol-agent: runtime not implemented yet (config=%s)\n", *configPath)
	os.Exit(78)
}
