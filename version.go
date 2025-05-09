package zstd

// Version information for the library
const (
	// Version is the semantic version of the library
	Version = "0.1.0"

	// CommitHash can be set during build using ldflags
	CommitHash = "unknown"
)

// VersionInfo returns a formatted string with version and build info
func VersionInfo() string {
	return Version + " (commit: " + CommitHash + ")"
}
