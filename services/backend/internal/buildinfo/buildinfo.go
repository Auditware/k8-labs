// Package buildinfo carries the build's identity so a running backend can say
// which commit it is. Stamped at link time via
//
//	-ldflags "-X backend/internal/buildinfo.SHA=$(git rev-parse --short HEAD)"
//
// in every place the binary is built (Tiltfile compile, the prod Dockerfile via
// a build-arg, CI). Surfaced at /healthz so "is this running the current code?"
// is a one-line check instead of a silent guess - the failure that froze local
// dev ~4.5h behind prod with no signal. Defaults to "dev" for an unstamped
// (bare `go build` / `go run`) build.
package buildinfo

// SHA is the short git commit the binary was built from, or "dev" if unstamped.
var SHA = "dev"
