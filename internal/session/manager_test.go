package session

import (
	"context"
	"testing"
	"time"
)

func TestCancelAndWaitWaitsForRuntimeToExit(t *testing.T) {
	manager := NewManager(nil)
	runtimeContext, cancel := context.WithCancel(context.Background())
	if err := manager.RegisterActive("session", cancel); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() {
		<-runtimeContext.Done()
		manager.Done("session")
		close(exited)
	}()

	if !manager.Cancel("session") {
		t.Fatal("Cancel should find the active runtime")
	}
	waitContext, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := manager.Wait(waitContext, "session"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("runtime did not exit before Wait returned")
	}
}
