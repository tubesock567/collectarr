package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const defaultLogEntryLimit = 500

type LogField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type LogEntry struct {
	Timestamp time.Time  `json:"timestamp"`
	Level     string     `json:"level"`
	Message   string     `json:"message"`
	Fields    []LogField `json:"fields"`
}

type LogsResponse struct {
	Entries []LogEntry `json:"entries"`
}

type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	start   int
	count   int
	limit   int
}

func NewLogBuffer(limit int) *LogBuffer {
	if limit <= 0 {
		limit = defaultLogEntryLimit
	}

	return &LogBuffer{
		entries: make([]LogEntry, limit),
		limit:   limit,
	}
}

func (b *LogBuffer) Append(record slog.Record, fields []LogField) {
	if b == nil {
		return
	}

	entry := LogEntry{
		Timestamp: record.Time.UTC(),
		Level:     record.Level.String(),
		Message:   record.Message,
		Fields:    make([]LogField, 0, len(fields)+record.NumAttrs()),
	}
	entry.Fields = append(entry.Fields, fields...)

	record.Attrs(func(attr slog.Attr) bool {
		entry.Fields = append(entry.Fields, flattenAttr(attr, nil)...)
		return true
	})

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count < b.limit {
		index := (b.start + b.count) % b.limit
		b.entries[index] = entry
		b.count++
		return
	}

	b.entries[b.start] = entry
	b.start = (b.start + 1) % b.limit
}

func (b *LogBuffer) List(limit int) []LogEntry {
	if b == nil {
		return nil
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	count := b.count
	if count == 0 {
		return []LogEntry{}
	}

	if limit <= 0 || limit > count {
		limit = count
	}

	entries := make([]LogEntry, 0, limit)
	for i := 0; i < limit; i++ {
		index := (b.start + count - 1 - i + b.limit) % b.limit
		entry := b.entries[index]
		fields := append([]LogField(nil), entry.Fields...)
		entries = append(entries, LogEntry{
			Timestamp: entry.Timestamp,
			Level:     entry.Level,
			Message:   entry.Message,
			Fields:    fields,
		})
	}

	return entries
}

type BufferedHandler struct {
	next   slog.Handler
	buffer *LogBuffer
	attrs  []slog.Attr
	groups []string
}

func NewBufferedHandler(next slog.Handler, buffer *LogBuffer) *BufferedHandler {
	return &BufferedHandler{next: next, buffer: buffer}
}

func (h *BufferedHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *BufferedHandler) Handle(ctx context.Context, record slog.Record) error {
	h.buffer.Append(record, h.flattenHandlerAttrs())
	return h.next.Handle(ctx, record)
}

func (h *BufferedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clonedAttrs := append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &BufferedHandler{next: h.next.WithAttrs(attrs), buffer: h.buffer, attrs: clonedAttrs, groups: append([]string(nil), h.groups...)}
}

func (h *BufferedHandler) WithGroup(name string) slog.Handler {
	clonedGroups := append(append([]string(nil), h.groups...), name)
	return &BufferedHandler{next: h.next.WithGroup(name), buffer: h.buffer, attrs: append([]slog.Attr(nil), h.attrs...), groups: clonedGroups}
}

func (h *BufferedHandler) flattenHandlerAttrs() []LogField {
	fields := make([]LogField, 0, len(h.attrs))
	for _, attr := range h.attrs {
		fields = append(fields, flattenAttr(attr, h.groups)...)
	}
	return fields
}

func formatLogValue(value slog.Value) string {
	resolved := value.Resolve()
	if resolved.Kind() == slog.KindGroup {
		return fmt.Sprint(resolved.Any())
	}
	return resolved.String()
}

func flattenAttr(attr slog.Attr, groups []string) []LogField {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) || attr.Key == "" && attr.Value.Kind() == slog.KindAny && attr.Value.Any() == nil {
		return nil
	}

	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if attr.Key != "" {
			nextGroups = append(append([]string(nil), groups...), attr.Key)
		}

		groupFields := make([]LogField, 0, len(attr.Value.Group()))
		for _, nested := range attr.Value.Group() {
			groupFields = append(groupFields, flattenAttr(nested, nextGroups)...)
		}
		return groupFields
	}

	key := attr.Key
	if len(groups) > 0 {
		if key != "" {
			key = strings.Join(append(append([]string(nil), groups...), key), ".")
		} else {
			key = strings.Join(groups, ".")
		}
	}

	return []LogField{{Key: key, Value: formatLogValue(attr.Value)}}
}
