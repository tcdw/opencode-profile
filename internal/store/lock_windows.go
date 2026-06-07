package store

import (
	"os"

	"golang.org/x/sys/windows"
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
	var ol windows.Overlapped
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &ol); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
		f.Close()
	}, nil
}
