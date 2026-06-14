package buildinfo

var (
	version = "dev"
	commit  = "unknown"
	built   = "unknown"
)

// String returns stable human-readable build metadata.
func String() string {
	return "heracles version=" + version + " commit=" + commit + " built=" + built
}

// Version returns the build's version, matching the release tag for tagged builds.
func Version() string {
	return version
}
