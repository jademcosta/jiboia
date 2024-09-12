package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/jademcosta/jiboia/pkg/config"
)

const (
	ComponentKey         = "component"
	FlowKey              = "flow"
	ExternalQueueTypeKey = "ext_queue_type"
	ObjStorageTypeKey    = "obj_storage_type"
)

func New(conf *config.LogConfig) *slog.Logger {
	level := generateLevel(conf.Level)

	var handler slog.Handler
	if conf.Format == "json" { //TODO: magic string
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	return slog.New(handler)
}

// Should be used only on tests
func NewDummy() *slog.Logger {
	return slog.New(&dummyHandler{})
}

func generateLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type dummyHandler struct {
}

func (h *dummyHandler) Enabled(context.Context, slog.Level) bool {
	return false
}
func (h *dummyHandler) Handle(context.Context, slog.Record) error {
	return nil
}
func (h *dummyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}
func (h *dummyHandler) WithGroup(name string) slog.Handler {
	return h
}
