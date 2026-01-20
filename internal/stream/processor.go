// Package stream provides real-time processing of Claude Code NDJSON responses.
package stream

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

// ansiCSI strips common ANSI escape sequences (CSI).
var ansiCSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// Options configures the stream processor behavior.
type Options struct {
	// ShowTools outputs tool invocation events when true.
	ShowTools bool

	// DebugUnhandled logs unhandled event types to stderr when true.
	DebugUnhandled bool

	// DebugWriter receives debug output for unhandled events.
	// If nil and DebugUnhandled is true, debug output is discarded.
	DebugWriter io.Writer
}

// Processor handles real-time streaming of Claude Code NDJSON output.
type Processor struct {
	opts Options
	out  *bufio.Writer
}

// NewProcessor creates a stream processor that writes to the given writer.
func NewProcessor(w io.Writer, opts Options) *Processor {
	return &Processor{
		opts: opts,
		out:  bufio.NewWriterSize(w, 64*1024),
	}
}

// Process reads from r and streams extracted text to the output writer.
// Returns an error if the stream cannot be read.
func (p *Processor) Process(r io.Reader) error {
	err := JsonObjects(r, func(raw []byte) {
		p.processObject(raw)
		_ = p.out.Flush() // real-time streaming
	})
	if err != nil && err != io.EOF {
		return err
	}
	return p.out.Flush()
}

// JsonObjects reads a byte stream and yields complete top-level JSON objects
// even if they are concatenated without newlines.
// It ignores any non-JSON noise between objects by waiting for the next '{'.
func JsonObjects(r io.Reader, onObject func([]byte)) error {
	br := bufio.NewReaderSize(r, 64*1024)

	var buf bytes.Buffer
	capturing := false
	depth := 0
	inStr := false
	esc := false

	for {
		b, err := br.ReadByte()
		if err != nil {
			return err
		}

		if !capturing {
			if b == '{' {
				capturing = true
				depth = 1
				inStr = false
				esc = false
				buf.Reset()
				buf.WriteByte(b)
			}
			continue
		}

		buf.WriteByte(b)

		if inStr {
			if esc {
				esc = false
				continue
			}
			if b == '\\' {
				esc = true
				continue
			}
			if b == '"' {
				inStr = false
			}
			continue
		}

		// Not in string
		switch b {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				raw := make([]byte, buf.Len())
				copy(raw, buf.Bytes())
				onObject(raw)

				capturing = false
				buf.Reset()
			}
		}
	}
}

func (p *Processor) processObject(raw []byte) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return
	}
	m, ok := v.(map[string]any)
	if !ok {
		return
	}

	typ := getString(m, "type")
	sub := getString(m, "subtype")

	// Tool-ish events
	if p.opts.ShowTools && looksLikeToolEvent(typ, sub, m) {
		name := firstNonEmpty(
			getString(m, "tool"),
			getString(m, "tool_name"),
			getString(m, "name"),
			getString(m, "command"),
		)
		if name == "" {
			name = "tool"
		}
		_, _ = p.out.WriteString("\n--- TOOL: ")
		_, _ = p.out.WriteString(Sanitize(name))
		_, _ = p.out.WriteString(" ---\n")
		if s := ExtractAnyText(m); s != "" {
			s = Sanitize(s)
			_, _ = p.out.WriteString(s)
			if !strings.HasSuffix(s, "\n") {
				_, _ = p.out.WriteString("\n")
			}
		}
		_, _ = p.out.WriteString("--- END TOOL ---\n")
		return
	}

	// Assistant text streaming
	text, mode := ExtractText(m)
	if text == "" {
		if p.opts.DebugUnhandled && p.opts.DebugWriter != nil {
			if typ != "" || sub != "" {
				_, _ = p.opts.DebugWriter.Write([]byte("UNHANDLED event type=" + typ + " subtype=" + sub + "\n"))
			}
		}
		return
	}

	text = Sanitize(text)
	_, _ = p.out.WriteString(text)
	if mode == "message" && !strings.HasSuffix(text, "\n") {
		_, _ = p.out.WriteString("\n")
	}
}

// Sanitize removes ANSI CSI sequences and control chars except \n, \t, \r.
func Sanitize(s string) string {
	s = ansiCSI.ReplaceAllString(s, "")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\n', '\t', '\r':
			b.WriteRune(r)
		default:
			// drop ASCII control chars; keep printable + non-ASCII runes
			if r >= 0x20 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// ExtractText extracts text content from a Claude event object.
// Returns the text and a mode indicating whether it's a delta or message.
func ExtractText(m map[string]any) (text string, mode string) {
	typ := getString(m, "type")
	sub := getString(m, "subtype")

	// Common wrapper keys used by various streamers.
	for _, k := range []string{"chunk", "data", "payload", "event", "part"} {
		if inner, ok := m[k].(map[string]any); ok {
			if s, md := ExtractText(inner); s != "" {
				return s, md
			}
		}
	}

	// 0) Explicit content_block (Anthropic-style)
	if cb, ok := m["content_block"].(map[string]any); ok {
		if s := getString(cb, "text"); s != "" {
			return s, modeFor(typ, sub, "message")
		}
		if s := getString(cb, "text_delta"); s != "" {
			return s, "delta"
		}
		if d, ok := cb["delta"].(map[string]any); ok {
			if s := getString(d, "text"); s != "" {
				return s, "delta"
			}
			if s := getString(d, "text_delta"); s != "" {
				return s, "delta"
			}
		}
	}

	// 1) Direct text fields
	if s := getString(m, "text"); s != "" {
		return s, modeFor(typ, sub, "message")
	}
	if s := getString(m, "text_delta"); s != "" {
		return s, "delta"
	}

	// 2) Delta object
	if d, ok := m["delta"].(map[string]any); ok {
		if s := getString(d, "text"); s != "" {
			return s, "delta"
		}
		if s := getString(d, "text_delta"); s != "" {
			return s, "delta"
		}
		if dd, ok := d["delta"].(map[string]any); ok {
			if s := getString(dd, "text"); s != "" {
				return s, "delta"
			}
			if s := getString(dd, "text_delta"); s != "" {
				return s, "delta"
			}
		}
	}

	// 3) Message object with content blocks
	if msg, ok := m["message"].(map[string]any); ok {
		if s := extractMessageContentText(msg); s != "" {
			return s, "message"
		}
		if d, ok := msg["delta"].(map[string]any); ok {
			if s := getString(d, "text"); s != "" {
				return s, "delta"
			}
			if s := getString(d, "text_delta"); s != "" {
				return s, "delta"
			}
		}
	}

	// 4) Top-level content
	if s := extractMessageContentText(m); s != "" {
		return s, modeFor(typ, sub, "message")
	}

	return "", ""
}

func modeFor(typ, sub, fallback string) string {
	if strings.Contains(typ, "delta") || strings.Contains(sub, "delta") {
		return "delta"
	}
	switch typ {
	case "content_block_delta", "delta", "text_delta", "message_delta":
		return "delta"
	}
	switch sub {
	case "content_block_delta", "delta", "text_delta", "message_delta":
		return "delta"
	}
	return fallback
}

func extractMessageContentText(m map[string]any) string {
	switch c := m["content"].(type) {
	case string:
		return c
	case []any:
		var b strings.Builder
		for _, it := range c {
			if blk, ok := it.(map[string]any); ok {
				if s := getString(blk, "text"); s != "" {
					b.WriteString(s)
				}
				if inner, ok := blk["content"].([]any); ok {
					for _, it2 := range inner {
						if blk2, ok := it2.(map[string]any); ok {
							if s := getString(blk2, "text"); s != "" {
								b.WriteString(s)
							}
						}
					}
				}
			}
		}
		return b.String()
	default:
		return ""
	}
}

// ExtractAnyText attempts to extract text from various fields in an event.
// Used for tool output and other non-standard event types.
func ExtractAnyText(m map[string]any) string {
	if s := getString(m, "text"); s != "" {
		return s
	}
	if s := getString(m, "text_delta"); s != "" {
		return s
	}
	if cb, ok := m["content_block"].(map[string]any); ok {
		if s := getString(cb, "text"); s != "" {
			return s
		}
		if s := getString(cb, "text_delta"); s != "" {
			return s
		}
	}
	if d, ok := m["delta"].(map[string]any); ok {
		if s := getString(d, "text"); s != "" {
			return s
		}
		if s := getString(d, "text_delta"); s != "" {
			return s
		}
	}
	if s := getString(m, "output"); s != "" {
		return s
	}
	if s := getString(m, "result"); s != "" {
		return s
	}
	if s := getString(m, "stderr"); s != "" {
		return s
	}
	if s := getString(m, "stdout"); s != "" {
		return s
	}
	if msg, ok := m["message"].(map[string]any); ok {
		if s := extractMessageContentText(msg); s != "" {
			return s
		}
	}
	if part, ok := m["part"].(map[string]any); ok {
		if s := ExtractAnyText(part); s != "" {
			return s
		}
	}
	return ""
}

func looksLikeToolEvent(typ, sub string, m map[string]any) bool {
	if typ == "tool" || typ == "tool_use" || typ == "tool_result" {
		return true
	}
	if sub == "tool" || sub == "tool_use" || sub == "tool_result" {
		return true
	}
	if _, ok := m["tool"]; ok {
		return true
	}
	if _, ok := m["tool_name"]; ok {
		return true
	}
	if _, ok := m["command"]; ok {
		return true
	}
	return false
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
