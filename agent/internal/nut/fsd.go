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

func ValidatePrimaryMonitor(path, upsName string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open upsmon config: %w", err)
	}
	defer f.Close()

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
		configuredUPS := fields[1]
		if at := strings.IndexByte(configuredUPS, '@'); at >= 0 {
			configuredUPS = configuredUPS[:at]
		}
		if configuredUPS != upsName {
			continue
		}
		role := strings.ToLower(fields[5])
		if role == "primary" || role == "master" {
			return nil
		}
	}
	if err := s.Err(); err != nil {
		return fmt.Errorf("scan upsmon config: %w", err)
	}
	return ErrNotPrimary
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
