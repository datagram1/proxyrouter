package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the application version
	Version = "dev"
	// Commit is the git commit hash
	Commit = "unknown"
	// BuildDate is the build date
	BuildDate = "unknown"
	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
)

// Info returns version information
func Info() string {
	return fmt.Sprintf("ProxyRouter %s (commit: %s, built: %s, go: %s)", 
		Version, Commit, BuildDate, GoVersion)
}

// Short returns short version information
func Short() string {
	return fmt.Sprintf("ProxyRouter %s", Version)
}
