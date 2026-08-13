package approval

import (
	"context"
	"testing"
	"time"
)

func TestApprovalRequiresMatchingHashAndResolvesOnce(t *testing.T) {
	manager := NewManager()
	request := manager.Create(Request{ToolName: "shell", Summary: "git status", Risk: "low"})
	done := make(chan Resolution, 1)
	go func() { result, _ := manager.Wait(context.Background(), request.ID); done <- result }()
	if err := manager.Resolve(Resolution{ApprovalID: request.ID, Status: Approved, RequestHash: "wrong"}); err == nil {
		t.Fatal("expected hash mismatch")
	}
	if err := manager.Resolve(Resolution{ApprovalID: request.ID, Status: Approved, RequestHash: request.RequestHash}); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.Status != Approved {
			t.Fatalf("result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("approval did not resolve")
	}
	if err := manager.Resolve(Resolution{ApprovalID: request.ID, Status: Approved, RequestHash: request.RequestHash}); err == nil {
		t.Fatal("expected second resolution error")
	}
}

func TestApprovalCancellation(t *testing.T) {
	manager := NewManager()
	request := manager.Create(Request{Summary: "write file"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := manager.Wait(ctx, request.ID); err == nil {
		t.Fatal("expected cancellation")
	}
}
