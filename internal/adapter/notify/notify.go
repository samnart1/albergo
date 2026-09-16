package notify

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/samnart1/albergo/internal/app/ports"
)

type LogSender struct {
	log *slog.Logger
}

func NewLogSender(log *slog.Logger) *LogSender { return &LogSender{log: log} }

func (s *LogSender) Send(ctx context.Context, msg ports.Message) error {
	var payload struct {
		Reference  string `json:"reference"`
		GuestEmail string `json:"guest_email"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "notification sent",
		"event", msg.Type,
		"reference", payload.Reference,
		"to", payload.GuestEmail,
	)
	return nil
}
