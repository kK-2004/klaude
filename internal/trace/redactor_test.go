package trace

import (
	"strings"
	"testing"
)

func TestRedactString(t *testing.T) {
	redacted := RedactString(`Authorization: Bearer secret api_key=sk-live-secret`)
	if strings.Contains(redacted, "secret") {
		t.Fatalf("secret leaked: %s", redacted)
	}
}

func TestRedactJSON(t *testing.T) {
	redacted := string(RedactJSON([]byte(`{"token":"secret","message":"safe"}`)))
	if strings.Contains(redacted, "secret") || !strings.Contains(redacted, "safe") {
		t.Fatalf("unexpected redacted JSON: %s", redacted)
	}
}

func TestTraceWriterRotatesAndRedacts(t *testing.T) {
	path := t.TempDir() + "/trace.jsonl"
	writer, err := Open(path, 150)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(Record{Type: "tool", Payload: map[string]any{"api_key": "secret"}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Write(Record{Type: "tool", Payload: map[string]any{"output": strings.Repeat("x", 200)}}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLines(path); err != nil {
		t.Fatal(err)
	}
}
