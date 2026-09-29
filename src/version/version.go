// Package version carries build-time identity, injected via -ldflags -X (see Dockerfile).
package version

var (
	// Version is the released version (git tag), e.g. "1.2.3".
	Version = "dev"
	// Commit is the short git SHA the binary was built from.
	Commit = "unknown"
	// BuildDate is the RFC3339 build timestamp.
	BuildDate = "unknown"
)

// String renders the identity line for logs/banners.
func String() string {
	return Version + " (" + Commit + ", " + BuildDate + ")"
}
