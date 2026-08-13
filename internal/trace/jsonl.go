package trace

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Writer 追加写入 JSONL 轨迹；超 maxBytes 时轮转到 path.<millis>，写入前脱敏。
type Writer struct {
	mu       sync.Mutex
	file     *os.File
	path     string
	maxBytes int64
	version  int
}

type Record struct {
	Version    int            `json:"version"`
	Type       string         `json:"type"`
	EventID    string         `json:"eventId,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
	Payload    map[string]any `json:"payload,omitempty"`
}

func Open(path string, maxBytes int64) (*Writer, error) {
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &Writer{file: file, path: path, maxBytes: maxBytes, version: 1}, nil
}

// Write 先脱敏再落盘；即将超限时轮转旧文件。
func (w *Writer) Write(record Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return errors.New("trace: writer is closed")
	}
	if record.Version == 0 {
		record.Version = w.version
	}
	if record.OccurredAt.IsZero() {
		record.OccurredAt = time.Now().UTC()
	}
	encoded := RedactJSON(mustJSON(record))
	encoded = append(encoded, '\n')
	if stat, err := w.file.Stat(); err == nil && stat.Size()+int64(len(encoded)) > w.maxBytes {
		if err := w.rotateLocked(); err != nil {
			return err
		}
	}
	if _, err := w.file.Write(encoded); err != nil {
		return err
	}
	return w.file.Sync()
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *Writer) rotateLocked() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	backup := fmt.Sprintf("%s.%d", w.path, time.Now().UnixMilli())
	if err := os.Rename(w.path, backup); err != nil {
		return err
	}
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	w.file = file
	return nil
}

func ReadLines(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var records []Record
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func mustJSON(value any) []byte { data, _ := json.Marshal(value); return data }
