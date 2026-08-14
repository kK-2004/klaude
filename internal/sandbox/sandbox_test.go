package sandbox_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/klaude/klaude/internal/sandbox"
)

func TestParseMode(t *testing.T) {
	mode, err := sandbox.ParseMode("workspace-write")
	if err != nil || mode != sandbox.ModeWorkspaceWrite {
		t.Fatalf("got %q %v", mode, err)
	}
	if _, err := sandbox.ParseMode("nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestClassifyDenialAndRunner(t *testing.T) {
	confined := sandbox.Confined{
		DenialSignatures: []string{"operation not permitted"},
		RunnerFailRules: []sandbox.RunnerFailureRule{{
			FatalSignatures: []string{"sandbox-exec:"},
		}},
	}
	if got := sandbox.Classify(1, "foo: Operation not permitted\n", confined); got != sandbox.OutcomeDenied {
		t.Fatalf("denied: %s", got)
	}
	if got := sandbox.Classify(1, "sandbox-exec: failed to load profile\n", confined); got != sandbox.OutcomeRunnerFailed {
		t.Fatalf("runner: %s", got)
	}
	if got := sandbox.Classify(1, "command blew up\n", confined); got != sandbox.OutcomeCommandFailed {
		t.Fatalf("command: %s", got)
	}
}

func TestUnavailableBackendFailClosed(t *testing.T) {
	backend := sandbox.Unavailable{Platform: "test"}
	ok, _, err := backend.Probe(context.Background())
	if ok || err == nil {
		t.Fatalf("probe ok=%v err=%v", ok, err)
	}
	var unavailable *sandbox.UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("expected UnavailableError, got %v", err)
	}
	_, err = backend.Confine(context.Background(), "echo", nil, sandbox.Policy{Mode: sandbox.ModeReadOnly, WorkspaceRoot: "/tmp"})
	if err == nil {
		t.Fatal("expected confine error")
	}
}

func TestPlatformSeatbeltOnDarwin(t *testing.T) {
	backend := sandbox.Platform()
	if runtime.GOOS != "darwin" {
		if backend.Name() == "seatbelt" {
			t.Fatalf("unexpected seatbelt on %s", runtime.GOOS)
		}
		return
	}
	if backend.Name() != "seatbelt" {
		t.Fatalf("name=%s", backend.Name())
	}
	ok, enforcement, err := backend.Probe(context.Background())
	if err != nil || !ok {
		// Nested CI/agent sandboxes often forbid sandbox-exec apply.
		t.Skipf("seatbelt probe unavailable in this environment: ok=%v err=%v", ok, err)
	}
	if enforcement != sandbox.EnforcementFull {
		t.Fatalf("enforcement=%s", enforcement)
	}
	dir := t.TempDir()

	confined, err := backend.Confine(context.Background(), "/bin/echo", []string{"hi"}, sandbox.Policy{
		Mode: sandbox.ModeWorkspaceWrite, WorkspaceRoot: dir, SessionID: "test-turn",
	})
	if err != nil {
		t.Fatal(err)
	}
	if confined.Program == "" || len(confined.Args) < 4 || confined.Args[0] != "-p" {
		t.Fatalf("confined=%+v", confined)
	}

	outsideDir := filepath.Join(t.TempDir(), "..", "outside-root")
	// Prefer a path under the user cache that is not covered by temp grants.
	if home, err := os.UserHomeDir(); err == nil {
		outsideDir = filepath.Join(home, "Library", "Caches", "klaude-sbx-test", filepath.Base(dir))
	}
	if err := os.MkdirAll(outsideDir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outsideDir, "deny.txt")
	_ = os.Remove(outside)
	defer os.RemoveAll(outsideDir)

	deny, err := backend.Confine(context.Background(), "/usr/bin/touch", []string{outside}, sandbox.Policy{
		Mode: sandbox.ModeWorkspaceWrite, WorkspaceRoot: dir, SessionID: "test-turn",
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(deny.Program, deny.Args...)
	cmd.Env = withEnv(deny.Env)
	output, runErr := cmd.CombinedOutput()
	if runErr == nil {
		t.Fatalf("expected outside touch to fail, output=%s", output)
	}
	if _, statErr := os.Stat(outside); statErr == nil {
		t.Fatalf("outside file was created despite sandbox: %s", outside)
	}

	inside := filepath.Join(dir, "ok.txt")
	allow, err := backend.Confine(context.Background(), "/usr/bin/touch", []string{inside}, sandbox.Policy{
		Mode: sandbox.ModeWorkspaceWrite, WorkspaceRoot: dir, SessionID: "test-turn",
	})
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(allow.Program, allow.Args...)
	cmd.Env = withEnv(allow.Env)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("inside touch failed: %v %s", err, output)
	}
	if _, err := os.Stat(inside); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxChainName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	backend := sandbox.Platform()
	ok, _, err := backend.Probe(context.Background())
	if !ok {
		t.Skipf("no bwrap/landlock in environment: %v", err)
	}
	name := backend.Name()
	if name != "bwrap" && name != "landlock" {
		t.Fatalf("unexpected backend %s", name)
	}
}

func withEnv(overrides map[string]string) []string {
	env := append([]string(nil), os.Environ()...)
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return env
}
