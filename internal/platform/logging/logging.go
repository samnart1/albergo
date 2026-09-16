package logging

import (
	"log/slog"
	"os"
)

func New(level, env string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	if env == "development" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}

	opts.ReplaceAttr = cloudLoggingKeys
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}

func cloudLoggingKeys(_ []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity"
	case slog.MessageKey:
		a.Key = "message"
	case slog.TimeKey:
		a.Key = "timestamp"
	}
	return a
}
