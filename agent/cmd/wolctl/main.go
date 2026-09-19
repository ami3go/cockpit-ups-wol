package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/ami3go/cockpit-ups-wol/agent/internal/wol"
)

func main() {
	fs := flag.NewFlagSet("wolctl", flag.ExitOnError)
	mac := fs.String("mac", "", "target MAC address")
	broadcast := fs.String("broadcast", "255.255.255.255", "IPv4 broadcast address")
	iface := fs.String("interface", "", "source network interface")
	port := fs.Int("port", 9, "UDP destination port")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: wolctl --mac AA:BB:CC:DD:EE:FF [options]\n\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(os.Args[1:])
	if *mac == "" {
		fs.Usage()
		os.Exit(2)
	}
	err := (wol.Sender{}).Wake(context.Background(), wol.Request{MAC: *mac, Broadcast: *broadcast, Interface: *iface, Port: *port})
	if err != nil {
		fmt.Fprintln(os.Stderr, "wolctl:", err)
		os.Exit(1)
	}
}
