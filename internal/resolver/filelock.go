package resolver

import (
	"io"
	"os"
	"syscall"
)

// fileLock wraps an os.File with POSIX advisory locking.
type fileLock struct {
	f *os.File
}

// Close releases the advisory lock and closes the underlying file.
func (l *fileLock) Close() error {
	// Unlock first, then close. Errors from unlock are secondary.
	unlockErr := syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	closeErr := l.f.Close()
	if closeErr != nil {
		return closeErr
	}
	return unlockErr
}

// AcquireLock opens (or creates) the file at path and acquires an exclusive
// POSIX advisory lock (flock). The returned io.Closer releases the lock.
// The lock file should be a sidecar (e.g. "data.lock"), not the data file
// itself, since the data file may be atomically replaced via rename.
func AcquireLock(path string) (io.Closer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close() // best-effort close on flock failure
		return nil, err
	}
	return &fileLock{f: f}, nil
}
