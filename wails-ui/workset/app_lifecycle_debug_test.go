package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/events"
)

func TestLifecycleApplicationEventName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event events.ApplicationEventType
		want  string
	}{
		{name: "common started", event: events.Common.ApplicationStarted, want: "common:ApplicationStarted"},
		{name: "mac did hide", event: events.Mac.ApplicationDidHide, want: "mac:ApplicationDidHide"},
		{name: "mac reopen", event: events.Mac.ApplicationShouldHandleReopen, want: "mac:ApplicationShouldHandleReopen"},
		{name: "fallback", event: events.ApplicationEventType(999999), want: "application:999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lifecycleApplicationEventName(tt.event); got != tt.want {
				t.Fatalf("lifecycleApplicationEventName(%d) = %q, want %q", tt.event, got, tt.want)
			}
		})
	}
}

func TestLifecycleWindowEventName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		event events.WindowEventType
		want  string
	}{
		{name: "common closing", event: events.Common.WindowClosing, want: "common:WindowClosing"},
		{name: "mac will close", event: events.Mac.WindowWillClose, want: "mac:WindowWillClose"},
		{name: "fallback", event: events.WindowEventType(999999), want: "window:999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := lifecycleWindowEventName(tt.event); got != tt.want {
				t.Fatalf("lifecycleWindowEventName(%d) = %q, want %q", tt.event, got, tt.want)
			}
		})
	}
}
