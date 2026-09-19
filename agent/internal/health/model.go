package health

import "time"

type State string

const (
	Healthy          State = "HEALTHY"
	Degraded         State = "DEGRADED"
	Recovering       State = "RECOVERING"
	ConfigValidating State = "CONFIG_VALIDATING"
	RollingBack      State = "ROLLING_BACK"
	FailedSafe       State = "FAILED_SAFE"
)

type Result struct {
	Name       string `json:"name"`
	OK         bool   `json:"ok"`
	Critical   bool   `json:"critical"`
	Repairable bool   `json:"repairable"`
	Message    string `json:"message,omitempty"`
}

type RepairRecord struct {
	Check   string `json:"check"`
	Attempt int    `json:"attempt"`
	Result  string `json:"result"`
	Error   string `json:"error,omitempty"`
}

type Circuit struct {
	Attempts       map[string]int       `json:"attempts"`
	NextAllowedUTC map[string]time.Time `json:"next_allowed_utc"`
	FailedReason   string               `json:"failed_safe_reason,omitempty"`
}

type Snapshot struct {
	Version    int            `json:"version"`
	State      State          `json:"state"`
	CheckedAt  time.Time      `json:"checked_at"`
	Results    []Result       `json:"results"`
	Repairs    []RepairRecord `json:"repairs,omitempty"`
	Circuit    Circuit        `json:"circuit"`
	Generation uint64         `json:"generation"`
}

func NewSnapshot() Snapshot {
	return Snapshot{Version: 1, State: Healthy, Circuit: Circuit{Attempts: map[string]int{}, NextAllowedUTC: map[string]time.Time{}}}
}
