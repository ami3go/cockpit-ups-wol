package config

import (
	"errors"
	"os"
	"syscall"
)

// tryLock attempts to take the configuration-history lock without waiting.
// It is used by agent startup because an intentional config activation or
// rollback may hold the lock while synchronously restarting the agent.
func (m *Manager) tryLock() (unlock func(), acquired bool, err error) {
	if err := m.Ensure(); err != nil {
		return nil, false, err
	}

	f, err := os.OpenFile(m.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, true, nil
}
