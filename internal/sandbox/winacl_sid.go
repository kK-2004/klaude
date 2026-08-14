package sandbox

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	sidKindWorkspace byte = 1
	sidKindTemp      byte = 2
)

// SyntheticSIDString derives a stable NT SID string from a kind+path pair.
// Authority 5 / RID base 110 is a private Klaude sandbox namespace (not a
// well-known Windows account). Used as WRITE_RESTRICTED restricting SIDs and
// matching ACE trustees on workspace/temp directories.
func SyntheticSIDString(kind byte, path string) string {
	cleaned := strings.ToLower(filepath.Clean(path))
	sum := sha256.Sum256(append([]byte{kind}, cleaned...))
	a := binary.LittleEndian.Uint32(sum[0:4])
	b := binary.LittleEndian.Uint32(sum[4:8])
	c := binary.LittleEndian.Uint32(sum[8:12])
	d := binary.LittleEndian.Uint32(sum[12:16])
	return fmt.Sprintf("S-1-5-110-%d-%d-%d-%d", a, b, c, d)
}

// WorkspaceWriteSIDString is the standing write capability for a workspace root.
func WorkspaceWriteSIDString(workspaceRoot string) string {
	return SyntheticSIDString(sidKindWorkspace, workspaceRoot)
}

// TempWriteSIDString is the revocable write capability for a session temp dir.
func TempWriteSIDString(tempDir string) string {
	return SyntheticSIDString(sidKindTemp, tempDir)
}
