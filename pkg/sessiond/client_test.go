package sessiond

import (
	"reflect"
	"testing"
)

func TestBuildSessiondCommandArgs(t *testing.T) {
	t.Run("adds_idle_timeout_when_set", func(t *testing.T) {
		opts := StartOptions{
			IdleTimeout: "0",
		}
		got := buildSessiondCommandArgs("/tmp/sessiond.sock", opts)
		want := []string{"--socket", "/tmp/sessiond.sock", "--idle-timeout", "0"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("omits_idle_timeout_when_empty", func(t *testing.T) {
		opts := StartOptions{}
		got := buildSessiondCommandArgs("/tmp/sessiond.sock", opts)
		want := []string{"--socket", "/tmp/sessiond.sock"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("trims_idle_timeout_whitespace", func(t *testing.T) {
		opts := StartOptions{
			IdleTimeout: "  0  ",
		}
		got := buildSessiondCommandArgs("/tmp/sessiond.sock", opts)
		want := []string{"--socket", "/tmp/sessiond.sock", "--idle-timeout", "0"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("includes_protocol_log_options", func(t *testing.T) {
		opts := StartOptions{
			ProtocolLogEnabled: true,
			ProtocolLogDir:     "/var/log",
			IdleTimeout:        "5m",
		}
		got := buildSessiondCommandArgs("/tmp/sessiond.sock", opts)
		want := []string{
			"--socket", "/tmp/sessiond.sock",
			"--verbose",
			"--protocol-log-dir", "/var/log",
			"--idle-timeout", "5m",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
}

