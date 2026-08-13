package trace

import (
	"io"
	"log/slog"
	"os"
)

func NewLogger(path string, level slog.Level) (*slog.Logger, io.Closer, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}
	return slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: level})), file, nil
}
