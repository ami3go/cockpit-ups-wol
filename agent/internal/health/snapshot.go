package health

// HealthSnapshot implements the IPC health-provider interface using the
// supervisor's durable snapshot. It does not trigger repairs as a side effect.
func (s *Supervisor) HealthSnapshot() (Snapshot, error) {
	return s.Load()
}
