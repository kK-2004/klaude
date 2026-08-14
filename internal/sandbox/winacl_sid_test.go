package sandbox_test

import (
	"strings"
	"testing"

	"github.com/kk-2004/klaude/internal/sandbox"
)

func TestSyntheticSIDStringStable(t *testing.T) {
	a := sandbox.WorkspaceWriteSIDString(`C:\proj`)
	b := sandbox.WorkspaceWriteSIDString(`C:\proj`)
	c := sandbox.WorkspaceWriteSIDString(`C:\other`)
	if a != b {
		t.Fatalf("unstable sid: %s vs %s", a, b)
	}
	if a == c {
		t.Fatal("different paths produced same SID")
	}
	if !strings.HasPrefix(a, "S-1-5-110-") {
		t.Fatalf("unexpected sid %s", a)
	}
	temp := sandbox.TempWriteSIDString(`C:\tmp\sess`)
	if temp == a {
		t.Fatal("workspace and temp SIDs collided")
	}
}
