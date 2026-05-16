package resolver

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func acquireLockWithRetry(path string, attempts int, delay time.Duration) (io.Closer, error) {
	var lastErr error
	for range attempts {
		lock, err := AcquireLock(path)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, err
		}
		lastErr = err
		time.Sleep(delay)
	}
	return nil, lastErr
}

func TestAcquireLockExclusive(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	const iterations = 20
	counterPath := filepath.Join(dir, "counter")
	if err := os.WriteFile(counterPath, []byte("0"), 0o644); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for range iterations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lock, err := acquireLockWithRetry(lockPath, 200, time.Millisecond)
			if err != nil {
				t.Errorf("AcquireLock: %v", err)
				return
			}
			defer lock.Close()

			data, err := os.ReadFile(counterPath)
			if err != nil {
				t.Errorf("ReadFile: %v", err)
				return
			}
			n := 0
			for _, b := range data {
				n = n*10 + int(b-'0')
			}
			n++
			s := []byte{0}
			if n >= 10 {
				s = make([]byte, 0, 3)
				tmp := n
				for tmp > 0 {
					s = append([]byte{byte('0' + tmp%10)}, s...)
					tmp /= 10
				}
			} else {
				s[0] = byte('0' + n)
			}
			if err := os.WriteFile(counterPath, s, 0o644); err != nil {
				t.Errorf("WriteFile: %v", err)
			}
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(counterPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "20"; got != want {
		t.Errorf("counter = %q, want %q (lost updates without proper locking)", got, want)
	}
}

func TestAcquireLockRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock1, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	if err := lock1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	lock2, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("second AcquireLock: %v", err)
	}
	if err := lock2.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestAcquireLockUsesPrivatePermissions(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	lock, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock.Close()

	info, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("lock mode = %o, want 600", got)
	}
}
