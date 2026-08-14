//go:build linux

package sandbox

import (
	"context"
	"sync"
)

// Platform returns Linux confinement: bwrap first, Landlock fallback.
func Platform() Backend {
	return &linuxChain{candidates: []Backend{&Bwrap{}, &Landlock{}}}
}

type linuxChain struct {
	candidates []Backend

	mu       sync.Mutex
	selected Backend
	probed   bool
	probeErr error
	enforce  Enforcement
}

func (c *linuxChain) Name() string {
	if c == nil {
		return "linux"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.selected != nil {
		return c.selected.Name()
	}
	return "linux"
}

func (c *linuxChain) Probe(ctx context.Context) (bool, Enforcement, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.probed {
		if c.selected == nil {
			return false, "", c.probeErr
		}
		return true, c.enforce, c.probeErr
	}
	c.probed = true
	var last error
	for _, candidate := range c.candidates {
		ok, enforcement, err := candidate.Probe(ctx)
		if ok {
			c.selected = candidate
			c.enforce = enforcement
			return true, enforcement, nil
		}
		if err != nil {
			last = err
		}
	}
	if last == nil {
		last = &UnavailableError{Reason: "bwrap and landlock probes both failed"}
	}
	c.probeErr = last
	return false, "", c.probeErr
}

func (c *linuxChain) Confine(ctx context.Context, program string, args []string, policy Policy) (Confined, error) {
	ok, _, err := c.Probe(ctx)
	if err != nil {
		return Confined{}, err
	}
	if !ok || c.selected == nil {
		return Confined{}, &UnavailableError{Reason: "no linux sandbox backend available"}
	}
	return c.selected.Confine(ctx, program, args, policy)
}
