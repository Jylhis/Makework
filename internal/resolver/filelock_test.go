package resolver

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAcquireLockExclusive(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Two goroutines increment a counter via file; with locking, no updates are lost.
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
			lock, err := AcquireLock(lockPath)
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
			// Write back
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
	got := string(data)
	want := "20"
	if got != want {
		t.Errorf("counter = %q, want %q (lost updates without proper locking)", got, want)
	}
}

func TestAcquireLockRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	// Acquire and release, then acquire again — should not deadlock.
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
