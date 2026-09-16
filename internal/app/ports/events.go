package ports

import (
	"context"
	"encoding/json"
	"uuid"
)

type Event struct {
	AggregateID uuid.UUID
	Type        string
	Payload     any
}

type Message struct {
	ID          int64
	AggregateID uuid.UUID
	Type        string
	Payload     json.RawMessage
	Attempts    int
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}

type OutboxRepository interface {
	Publish(ctx context.Context, event Event) error
	Dispatch(ctx context.Context, batch int, send func(ctx context.Context, msg Message) error) (int, error)
}

type Claim struct {
	Fresh         bool
	RequestHash   string
	ReservationID *uuid.UUID
}

type IdempotencyRepository interface {
	Claim(ctx context.Context, key, requestHash string) (Claim, error)
	Record(ctx context.Context, key string, reservationID uuid.UUID) error
}
