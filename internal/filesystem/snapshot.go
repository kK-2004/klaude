package filesystem

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SnapshotStore 以内容哈希做去重缓存，为写盘/补丁/撤销提供 before/after 基线。
type SnapshotStore struct{ Root string }
type Snapshot struct {
	ID          string
	Exists      bool
	Hash        string
	ContentPath string
	Mode        os.FileMode
	CreatedAt   time.Time
}
type Change struct {
	Path         string
	Status       string
	BeforeHash   string
	AfterHash    string
	Before       Snapshot
	Diff         string
	AddedLines   int
	DeletedLines int
}

var ErrStaleBaseline = errors.New("filesystem: stale file baseline")
var ErrAmbiguousPatch = errors.New("filesystem: patch context is ambiguous")

func NewSnapshotStore(root string) (SnapshotStore, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return SnapshotStore{}, err
	}
	return SnapshotStore{Root: root}, nil
}

// Capture 读取当前文件内容；不存在则 Exists=false。相同哈希只落盘一次。
func (s SnapshotStore) Capture(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	now := time.Now().UTC()
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{ID: newSnapshotID(), Exists: false, CreatedAt: now}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	contentPath := filepath.Join(s.Root, hash)
	if _, err := os.Stat(contentPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(contentPath, data, 0o600); err != nil {
			return Snapshot{}, err
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{ID: newSnapshotID(), Exists: true, Hash: hash, ContentPath: contentPath, Mode: info.Mode().Perm(), CreatedAt: now}, nil
}

func (s SnapshotStore) Restore(snapshot Snapshot, target string) error {
	if !snapshot.Exists {
		return os.Remove(target)
	}
	data, err := os.ReadFile(snapshot.ContentPath)
	if err != nil {
		return err
	}
	if snapshot.Mode == 0 {
		snapshot.Mode = 0o600
	}
	return atomicWrite(target, data, snapshot.Mode)
}

// WriteFile 先 Capture before，再按 expectedHash 做乐观锁校验，最后原子写入并生成 Change。
func (s Service) WriteFile(store SnapshotStore, target string, content []byte, expectedHash string) (Change, error) {
	path, err := s.Resolve(target, true)
	if err != nil {
		return Change{}, err
	}
	before, err := store.Capture(path)
	if err != nil {
		return Change{}, err
	}
	if expectedHash != "" && before.Hash != expectedHash {
		return Change{}, ErrStaleBaseline
	}
	if before.Exists && before.Hash == hashBytes(content) {
		return Change{Path: target, Status: "no_change", BeforeHash: before.Hash, AfterHash: before.Hash, Before: before}, nil
	}
	mode := before.Mode
	if mode == 0 {
		mode = 0o600
	}
	if err := atomicWrite(path, content, mode); err != nil {
		return Change{}, err
	}
	after := hashBytes(content)
	return buildChange(target, before, after, content)
}

// ApplyPatch 要求 oldText 在文件中恰好出现一次，避免模糊补丁误改多处。
func (s Service) ApplyPatch(store SnapshotStore, target, oldText, newText, expectedHash string) (Change, error) {
	path, err := s.Resolve(target, true)
	if err != nil {
		return Change{}, err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return Change{}, err
	}
	before, err := store.Capture(path)
	if err != nil {
		return Change{}, err
	}
	if expectedHash != "" && before.Hash != expectedHash {
		return Change{}, ErrStaleBaseline
	}
	content := string(current)
	count := strings.Count(content, oldText)
	if count == 0 {
		return Change{}, ErrStaleBaseline
	}
	if count > 1 {
		return Change{}, ErrAmbiguousPatch
	}
	updated := strings.Replace(content, oldText, newText, 1)
	return s.WriteFile(store, target, []byte(updated), before.Hash)
}

func buildChange(path string, before Snapshot, afterHash string, after []byte) (Change, error) {
	beforeData := []byte{}
	if before.Exists {
		var err error
		beforeData, err = os.ReadFile(before.ContentPath)
		if err != nil {
			return Change{}, err
		}
	}
	diff := UnifiedDiff(path, string(beforeData), string(after))
	added, deleted := lineStats(string(beforeData), string(after))
	status := "modified"
	if !before.Exists {
		status = "added"
	}
	return Change{Path: path, Status: status, BeforeHash: before.Hash, AfterHash: afterHash, Before: before, Diff: diff, AddedLines: added, DeletedLines: deleted}, nil
}

// UnifiedDiff 生成简化统一 diff：去掉公共前后缀后标 +/-，供 UI 与审计展示。
func UnifiedDiff(path, before, after string) string {
	if before == after {
		return ""
	}
	beforeLines := strings.Split(strings.TrimSuffix(before, "\n"), "\n")
	afterLines := strings.Split(strings.TrimSuffix(after, "\n"), "\n")
	if before == "" {
		beforeLines = nil
	}
	if after == "" {
		afterLines = nil
	}
	prefix := 0
	for prefix < len(beforeLines) && prefix < len(afterLines) && beforeLines[prefix] == afterLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(beforeLines)-prefix && suffix < len(afterLines)-prefix && beforeLines[len(beforeLines)-1-suffix] == afterLines[len(afterLines)-1-suffix] {
		suffix++
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "--- a/%s\n+++ b/%s\n@@\n", path, path)
	for _, line := range beforeLines[:prefix] {
		builder.WriteString(" ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	for _, line := range beforeLines[prefix : len(beforeLines)-suffix] {
		builder.WriteString("-")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	for _, line := range afterLines[prefix : len(afterLines)-suffix] {
		builder.WriteString("+")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	for _, line := range beforeLines[len(beforeLines)-suffix:] {
		builder.WriteString(" ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func lineStats(before, after string) (int, int) {
	beforeLines, afterLines := len(strings.Split(strings.TrimSuffix(before, "\n"), "\n")), len(strings.Split(strings.TrimSuffix(after, "\n"), "\n"))
	if before == "" {
		beforeLines = 0
	}
	if after == "" {
		afterLines = 0
	}
	if afterLines > beforeLines {
		return afterLines - beforeLines, 0
	}
	if beforeLines > afterLines {
		return 0, beforeLines - afterLines
	}
	return 1, 1
}

// atomicWrite 先写临时文件再 rename，降低半写入损坏风险。
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".klaude-write-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func hashBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func newSnapshotID() string { return hashBytes([]byte(time.Now().UTC().String()))[:32] }
