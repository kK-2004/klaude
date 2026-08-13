package filesystem

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrUndoConflict = errors.New("filesystem: undo conflict")

// Undo 先校验当前文件仍等于变更后的 AfterHash（否则认为有外部修改），
// 再逆序 Restore before 快照；原先不存在的文件则删除。
func (s Service) Undo(store SnapshotStore, changes []Change) error {
	for _, change := range changes {
		path, err := s.Resolve(change.Path, true)
		if err != nil {
			return err
		}
		current, err := store.Capture(path)
		if err != nil {
			return err
		}
		if current.Hash != change.AfterHash || (!current.Exists && change.AfterHash != "") {
			return ErrUndoConflict
		}
	}
	for index := len(changes) - 1; index >= 0; index-- {
		change := changes[index]
		path, _ := s.Resolve(change.Path, true)
		if change.Before.Exists {
			if err := store.Restore(change.Before, path); err != nil {
				return err
			}
		} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func SnapshotPath(root, relative string) string { return filepath.Join(root, relative) }
