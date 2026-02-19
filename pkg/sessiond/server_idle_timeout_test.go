package sessiond

import (
	"testing"
	"time"
)

func TestNewServerUsesDefaultIdleTimeoutWhenUnset(t *testing.T) {
	opts := Options{}
	server := NewServer(opts)

	if server.opts.IdleTimeout != DefaultOptions().IdleTimeout {
		t.Fatalf("idle timeout = %s, want %s", server.opts.IdleTimeout, DefaultOptions().IdleTimeout)
	}
}

func TestNewServerKeepsExplicitZeroIdleTimeout(t *testing.T) {
	opts := Options{
		IdleTimeout:    0,
		IdleTimeoutSet: true,
	}
	server := NewServer(opts)

	if server.opts.IdleTimeout != 0 {
		t.Fatalf("idle timeout = %s, want disabled (0)", server.opts.IdleTimeout)
	}
}

func TestNewServerKeepsExplicitPositiveIdleTimeout(t *testing.T) {
	opts := Options{
		IdleTimeout:    5 * time.Minute,
		IdleTimeoutSet: true,
	}
	server := NewServer(opts)

	if server.opts.IdleTimeout != 5*time.Minute {
		t.Fatalf("idle timeout = %s, want %s", server.opts.IdleTimeout, 5*time.Minute)
	}
}
