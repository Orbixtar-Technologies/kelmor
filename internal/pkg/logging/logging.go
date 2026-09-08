package logging

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

var secretRE = regexp.MustCompile(`(?i)(password|passwd|secret|token|authorization|private[_-]?key|api[_-]?key|session)=([^\s&]+)`)

type Logger struct {
	mu      sync.Mutex
	out     io.Writer
	service string
}

func New(service string) *Logger {
	return &Logger{out: os.Stdout, service: service}
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

func (l *Logger) Info(ctx context.Context, event string, fields map[string]any) {
	l.write(ctx, "info", event, fields)
}

func (l *Logger) Error(ctx context.Context, event string, fields map[string]any) {
	l.write(ctx, "error", event, fields)
}

func (l *Logger) write(ctx context.Context, level, event string, fields map[string]any) {
	rec := map[string]any{
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"level":      level,
		"service":    l.service,
		"event":      event,
		"request_id": RequestID(ctx),
	}
	for k, v := range fields {
		if isSensitiveKey(k) {
			rec[k] = "[redacted]"
			continue
		}
		rec[k] = redactValue(v)
	}
	b, _ := json.Marshal(rec)
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(append(b, '\n'))
}

func isSensitiveKey(k string) bool {
	k = strings.ToLower(k)
	for _, p := range []string{"password", "secret", "token", "private_key", "session", "authorization", "api_key"} {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

func redactValue(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	return secretRE.ReplaceAllString(s, "${1}=[redacted]")
}
