package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/version"
)

func main() {
	fs := flag.NewFlagSet("cockpit-ups-wolctl", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showVersion := fs.Bool("version", false, "print version information and exit")
	socketPath := fs.String("socket", "/run/cockpit-ups-wol/agent.sock", "agent Unix socket")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: cockpit-ups-wolctl [options] <command>\n\n")
		fmt.Fprintf(fs.Output(), "Local CLI client for cockpit-ups-wol-agent.\n\nOptions:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nCommands:\n  status   Show agent status (planned)\n  health   Show health status (planned)\n")
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showVersion {
		b, _ := json.Marshal(version.Current())
		fmt.Println(string(b))
		return
	}
	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(2)
	}

	fmt.Fprintf(os.Stderr, "cockpit-ups-wolctl: command %q not implemented yet (socket=%s)\n", fs.Arg(0), *socketPath)
	os.Exit(78)
}
