package state

const Version = 1

type PowerState string

const (
	BootReconcile      PowerState = "BOOT_RECONCILE"
	Normal             PowerState = "NORMAL"
	OnBattery          PowerState = "ON_BATTERY"
	ShutdownCommitted  PowerState = "SHUTDOWN_COMMITTED"
	ShutdownInProgress PowerState = "SHUTDOWN_IN_PROGRESS"
	WaitingForAC       PowerState = "WAITING_FOR_AC"
	RecoveryWait       PowerState = "RECOVERY_WAIT"
	RecoveryStarted    PowerState = "RECOVERY_STARTED"
	RestoreHosts       PowerState = "RESTORE_HOSTS"
	FailedSafe         PowerState = "FAILED_SAFE"
)

type ShutdownState string

const (
	ShutdownNotRequired  ShutdownState = "not_required"
	ShutdownPlanned      ShutdownState = "planned"
	ShutdownRequested    ShutdownState = "requested"
	ShutdownAcknowledged ShutdownState = "acknowledged"
	ShutdownCompleted    ShutdownState = "completed"
	ShutdownUnknown      ShutdownState = "unknown"
	ShutdownFailed       ShutdownState = "failed"
)

type RecoveryState string

const (
	RecoveryNotRequired RecoveryState = "not_required"
	RecoveryWaiting     RecoveryState = "waiting"
	RecoveryWOLSent     RecoveryState = "wol_sent"
	RecoveryOnline      RecoveryState = "online"
	RecoveryUnknown     RecoveryState = "unknown"
	RecoveryFailed      RecoveryState = "failed"
)

type UPSObservation struct {
	Utility               string   `json:"utility,omitempty"`
	LowBattery            *bool    `json:"low_battery,omitempty"`
	FSD                   *bool    `json:"fsd,omitempty"`
	BatteryCharge         *float64 `json:"battery_charge,omitempty"`
	BatteryRuntimeSeconds *int64   `json:"battery_runtime_seconds,omitempty"`
	RawStatus             string   `json:"raw_status,omitempty"`
	ObservedAtWallclock   string   `json:"observed_at_wallclock,omitempty"`
}

type HostState struct {
	WasOnline        *bool         `json:"was_online"`
	ShutdownState    ShutdownState `json:"shutdown_state"`
	RecoveryState    RecoveryState `json:"recovery_state"`
	ShutdownAttempts int           `json:"shutdown_attempts"`
	WakeAttempts     int           `json:"wake_attempts"`
	LastActionID     string        `json:"last_action_id,omitempty"`
	LastVerification string        `json:"last_verification,omitempty"`
	LastError        string        `json:"last_error,omitempty"`
}

type ConfigTransaction struct {
	CandidateRevision string `json:"candidate_revision,omitempty"`
	Status            string `json:"status,omitempty"`
	LastKnownGood     string `json:"last_known_good,omitempty"`
}

type State struct {
	StateVersion         int                  `json:"state_version"`
	TransactionID        string               `json:"transaction_id"`
	ParentTransactionID  string               `json:"parent_transaction_id,omitempty"`
	Sequence             uint64               `json:"sequence"`
	PowerState           PowerState           `json:"power_state"`
	ShutdownCommitted    bool                 `json:"shutdown_committed"`
	RecoveryStarted      bool                 `json:"recovery_started"`
	ActiveConfigRevision string               `json:"active_config_revision"`
	LastUPS              *UPSObservation      `json:"last_ups,omitempty"`
	Hosts                map[string]HostState `json:"hosts"`
	ConfigTransaction    *ConfigTransaction   `json:"config_transaction,omitempty"`
	FailedSafeReason     string               `json:"failed_safe_reason,omitempty"`
	Checksum             string               `json:"checksum,omitempty"`
}

func New(transactionID, configRevision string) State {
	return State{
		StateVersion:         Version,
		TransactionID:        transactionID,
		PowerState:           BootReconcile,
		ActiveConfigRevision: configRevision,
		Hosts:                map[string]HostState{},
	}
}
