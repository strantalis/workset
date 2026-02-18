package sessiond

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestRepeatedCreateStop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported on windows")
	}

	client, cleanup := startTestServer(t)
	defer cleanup()

	const sessionID = "churn-test"
	const iterations = 40

	for i := 0; i < iterations; i++ {
		createCtx, createCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := client.Create(createCtx, sessionID, "/tmp")
		createCancel()
		if err != nil {
			t.Fatalf("create session iteration %d: %v", i, err)
		}

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = client.Stop(stopCtx, sessionID)
		stopCancel()
		if err != nil {
			t.Fatalf("stop session iteration %d: %v", i, err)
		}

		if !waitForSessionGone(t, client, sessionID, 3*time.Second) {
			t.Fatalf("session still present after stop iteration %d", i)
		}
	}
}
