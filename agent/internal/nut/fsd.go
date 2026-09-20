package nut

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

var ErrNotPrimary = errors.New("upsmon configuration does not declare this UPS primary")
var ErrMultiplePrimaries = errors.New("upsmon configuration declares multiple primary UPS monitors")

func ValidatePrimaryMonitor(path, upsName string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open upsmon config: %w", err)
	}
	defer f.Close()

	primaryCount := 0
	targetPrimary := false
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 || !strings.EqualFold(fields[0], "MONITOR") {
			continue
		}
		role := strings.ToLower(fields[5])
		if role != "primary" && role != "master" {
			continue
		}
		primaryCount++
		configuredUPS := fields[1]
		if at := strings.IndexByte(configuredUPS, '@'); at >= 0 {
			configuredUPS = configuredUPS[:at]
		}
		if configuredUPS == upsName {
			targetPrimary = true
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("scan upsmon config: %w", err)
	}
	if !targetPrimary {
		return ErrNotPrimary
	}
	// `upsmon -c fsd` is process-wide for UPSes this upsmon monitors as primary.
	// Until multi-UPS FSD ownership is explicitly modeled and hardware-tested,
	// fail closed rather than accidentally setting FSD on another primary UPS.
	if primaryCount != 1 {
		return fmt.Errorf("%w: found %d primary/master MONITOR entries", ErrMultiplePrimaries, primaryCount)
	}
	return nil
}

func (c *Client) RequestFSD(ctx context.Context, upsName, upsmonConfPath string) error {
	if err := ValidatePrimaryMonitor(upsmonConfPath, upsName); err != nil {
		return err
	}
	if c.Runner == nil {
		return errors.New("nut runner is nil")
	}
	if c.UPSMonPath == "" {
		c.UPSMonPath = "upsmon"
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := c.Runner.Run(ctx, c.UPSMonPath, "-c", "fsd")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("upsmon -c fsd failed: %w", err)
		}
		return fmt.Errorf("upsmon -c fsd failed: %w: %s", err, msg)
	}
	return nil
}
