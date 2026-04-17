// Package buildinfo exposes build-time metadata overridable via -ldflags.
package buildinfo

// Version is the binary version. Overridden at build time via:
//
//	go build -ldflags "-X github.com/jylhis/makework/internal/buildinfo.Version=x.y.z"
var Version = "0.1.0"
