package nut

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type UtilityState string

const (
	UtilityOnline    UtilityState = "ONLINE"
	UtilityOnBattery UtilityState = "ON_BATTERY"
	UtilityUnknown   UtilityState = "UNKNOWN"
)

type Status struct {
	Utility        UtilityState
	LowBattery     bool
	FSD            bool
	ChargePercent  *float64
	RuntimeSeconds *int64
	RawStatus      string
	Variables      map[string]string
}

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

type Client struct {
	Runner     Runner
	UPSCPath   string
	UPSMonPath string
	Timeout    time.Duration
}

func NewClient() *Client {
	return &Client{
		Runner:     ExecRunner{},
		UPSCPath:   "upsc",
		UPSMonPath: "upsmon",
		Timeout:    5 * time.Second,
	}
}

func Target(upsName, host string, port int) string {
	if host == "" {
		host = "localhost"
	}
	if port <= 0 || port == 3493 {
		return fmt.Sprintf("%s@%s", upsName, host)
	}
	return fmt.Sprintf("%s@%s:%d", upsName, host, port)
}

func (c *Client) Query(ctx context.Context, target string) (Status, error) {
	if c.Runner == nil {
		return Status{Utility: UtilityUnknown}, errors.New("nut runner is nil")
	}
	upscPath := c.UPSCPath
	if upscPath == "" {
		upscPath = "upsc"
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := c.Runner.Run(ctx, upscPath, target)
	if err != nil {
		return Status{Utility: UtilityUnknown}, fmt.Errorf("upsc %s failed: %w", target, err)
	}
	st, err := ParseUPSC(out)
	if err != nil {
		st.Utility = UtilityUnknown
		return st, err
	}
	return st, nil
}

func ParseUPSC(data []byte) (Status, error) {
	vars := make(map[string]string)
	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		vars[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if err := s.Err(); err != nil {
		return Status{Utility: UtilityUnknown, Variables: vars}, err
	}

	raw, ok := vars["ups.status"]
	if !ok || strings.TrimSpace(raw) == "" {
		return Status{Utility: UtilityUnknown, Variables: vars}, errors.New("ups.status missing")
	}
	st := Status{Utility: UtilityUnknown, RawStatus: raw, Variables: vars}
	tokens := strings.Fields(raw)
	var hasOL, hasOB bool
	for _, token := range tokens {
		switch token {
		case "OL":
			hasOL = true
		case "OB":
			hasOB = true
		case "LB":
			st.LowBattery = true
		case "FSD":
			st.FSD = true
		}
	}
	switch {
	case hasOL && !hasOB:
		st.Utility = UtilityOnline
	case hasOB && !hasOL:
		st.Utility = UtilityOnBattery
	default:
		st.Utility = UtilityUnknown
	}

	if v, ok := vars["battery.charge"]; ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			st.ChargePercent = &f
		}
	}
	if v, ok := vars["battery.runtime"]; ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			st.RuntimeSeconds = &n
		}
	}
	return st, nil
}
