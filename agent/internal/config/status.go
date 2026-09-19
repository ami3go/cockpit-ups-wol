package config

// PreviousKnownGood returns the revision immediately preceding last-known-good.
func (m *Manager) PreviousKnownGood() (string, error) {
	return m.readPointer("previous-known-good")
}
