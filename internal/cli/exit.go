package cli

import (
	"fmt"
	"os"
)

// Die prints "Error: <msg>" to stderr and exits with status 1.
func Die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
