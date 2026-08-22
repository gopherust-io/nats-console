package apikit

import (
	"bufio"
	"bytes"
)

// WriteSSEEvent writes a named Server-Sent Event. Payload may contain newlines
// (NATS monitoring endpoints return indented JSON); each line is emitted as its
// own "data:" field so EventSource reconstructs the full payload. A bare
// "data: {\\n ... }" without per-line prefixes leaves browsers with only "{".
func WriteSSEEvent(w *bufio.Writer, event string, payload []byte) error {
	if w == nil || len(payload) == 0 {
		return nil
	}
	if _, err := w.WriteString("event: "); err != nil {
		return err
	}
	if _, err := w.WriteString(event); err != nil {
		return err
	}
	if _, err := w.WriteString("\n"); err != nil {
		return err
	}

	// Trim a single trailing newline so we do not emit an empty final data line.
	payload = bytes.TrimSuffix(payload, []byte("\n"))
	payload = bytes.TrimSuffix(payload, []byte("\r"))

	start := 0
	for i := 0; i <= len(payload); i++ {
		if i < len(payload) && payload[i] != '\n' {
			continue
		}
		line := payload[start:i]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if _, err := w.WriteString("data: "); err != nil {
			return err
		}
		if _, err := w.Write(line); err != nil {
			return err
		}
		if _, err := w.WriteString("\n"); err != nil {
			return err
		}
		start = i + 1
	}
	if _, err := w.WriteString("\n"); err != nil {
		return err
	}
	return w.Flush()
}
