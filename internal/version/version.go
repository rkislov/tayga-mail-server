package version

import "fmt"

// Set via -ldflags "-X github.com/tayga/tms/internal/version.Version=…"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a short version line for CLI output.
func String() string {
	return fmt.Sprintf("tayga-mail %s (%s) built %s", Version, Commit, Date)
}
