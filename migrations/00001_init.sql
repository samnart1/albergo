-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE properties (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    city       text NOT NULL,
    timezone   text NOT NULL DEFAULT 'Europe/Rome',
    currency   char(3) NOT NULL DEFAULT 'EUR',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE room_types (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id   uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    code          text NOT NULL,
    name          text NOT NULL,
    max_occupancy smallint NOT NULL CHECK (max_occupancy BETWEEN 1 AND 10),
    UNIQUE (property_id, code)
);

CREATE TABLE rate_plans (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id            uuid NOT NULL REFERENCES properties(id) ON DELETE CASCADE,
    room_type_id           uuid NOT NULL REFERENCES room_types(id) ON DELETE CASCADE,
    code                   text NOT NULL,
    name                   text NOT NULL,
    refundable_until_hours int NOT NULL DEFAULT 24 CHECK (refundable_until_hours >= 0),
    UNIQUE (property_id, code)
);

CREATE TABLE rate_calendar (
    rate_plan_id        uuid NOT NULL REFERENCES rate_plans(id) ON DELETE CASCADE,
    stay_date           date NOT NULL,
    price_cents         bigint NOT NULL CHECK (price_cents >= 0),
    min_stay            smallint NOT NULL DEFAULT 1 CHECK (min_stay >= 1),
    closed_to_arrival   boolean NOT NULL DEFAULT false,
    closed_to_departure boolean NOT NULL DEFAULT false,
    PRIMARY KEY (rate_plan_id, stay_date)
);

CREATE TABLE inventory (
    room_type_id uuid NOT NULL REFERENCES room_types(id) ON DELETE CASCADE,
    stay_date    date NOT NULL,
    allotment    smallint NOT NULL CHECK (allotment >= 0),
    booked       smallint NOT NULL DEFAULT 0,
    PRIMARY KEY (room_type_id, stay_date),
    CONSTRAINT inventory_not_oversold CHECK (booked >= 0 AND booked <= allotment)
);

CREATE TABLE guests (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      text NOT NULL,
    full_name  text NOT NULL,
    phone      text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX guests_email_key ON guests (lower(email));

CREATE TYPE reservation_status AS ENUM ('confirmed', 'cancelled');

CREATE TABLE reservations (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    reference    text NOT NULL UNIQUE,
    property_id  uuid NOT NULL REFERENCES properties(id),
    room_type_id uuid NOT NULL REFERENCES room_types(id),
    rate_plan_id uuid NOT NULL REFERENCES rate_plans(id),
    guest_id     uuid NOT NULL REFERENCES guests(id),
    check_in     date NOT NULL,
    check_out    date NOT NULL,
    guest_count  smallint NOT NULL CHECK (guest_count >= 1),
    status       reservation_status NOT NULL DEFAULT 'confirmed',
    total_cents  bigint NOT NULL CHECK (total_cents >= 0),
    currency     char(3) NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    cancelled_at timestamptz,
    CONSTRAINT reservations_stay_valid CHECK (check_out > check_in),
    CONSTRAINT reservations_cancel_consistent
        CHECK ((status = 'cancelled') = (cancelled_at IS NOT NULL))
);
CREATE INDEX reservations_property_checkin_idx ON reservations (property_id, check_in);

CREATE TABLE reservation_nights (
    reservation_id uuid NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    stay_date      date NOT NULL,
    price_cents    bigint NOT NULL CHECK (price_cents >= 0),
    PRIMARY KEY (reservation_id, stay_date)
);

CREATE TABLE outbox (
    id           bigserial PRIMARY KEY,
    aggregate_id uuid NOT NULL,
    type         text NOT NULL,
    payload      jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    attempts     int NOT NULL DEFAULT 0,
    last_error   text
);
CREATE INDEX outbox_unpublished_idx ON outbox (created_at) WHERE published_at IS NULL;

CREATE TABLE idempotency_keys (
    key            text PRIMARY KEY,
    request_hash   text NOT NULL,
    status_code    int NOT NULL,
    response_body  jsonb NOT NULL,
    reservation_id uuid REFERENCES reservations(id),
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE idempotency_keys;
DROP TABLE outbox;
DROP TABLE reservation_nights;
DROP TABLE reservations;
DROP TYPE reservation_status;
DROP TABLE guests;
DROP TABLE inventory;
DROP TABLE rate_calendar;
DROP TABLE rate_plans;
DROP TABLE room_types;
DROP TABLE properties;
