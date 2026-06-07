package store

import (
	"os"
	"syscall"
)

// lock takes an exclusive advisory lock on Root/.lock so two ocp processes can't
// mutate the store concurrently. The returned func releases it. Held only around
// public mutating ops; Save runs under an already-held lock.
func (s *Store) lock() (func(), error) {
	if err := os.MkdirAll(s.layout.Root, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.layout.Lock(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
