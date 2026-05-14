// Package fsx holds small filesystem helpers shared across packages.
package fsx

import "os"

// PathExists returns true when path refers to anything reachable via
// os.Stat (file, directory, symlink target, ...). Errors are treated
// as "doesn't exist" to mirror the historical fileExists helper that
// lived in multiple packages.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
