package sandbox

import "context"

// Unavailable is a fail-closed placeholder used until a platform backend ships.
type Unavailable struct {
	Platform string
}

func (u Unavailable) Name() string {
	if u.Platform == "" {
		return "unavailable"
	}
	return "unavailable:" + u.Platform
}

func (Unavailable) Probe(ctx context.Context) (bool, Enforcement, error) {
	_ = ctx
	return false, "", &UnavailableError{Reason: "no sandbox backend for this platform yet"}
}

func (u Unavailable) Confine(ctx context.Context, program string, args []string, policy Policy) (Confined, error) {
	_ = ctx
	_ = program
	_ = args
	_ = policy
	return Confined{}, &UnavailableError{Backend: u.Name(), Reason: "no sandbox backend for this platform yet"}
}
