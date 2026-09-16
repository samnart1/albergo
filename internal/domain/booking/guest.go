package booking

import (
	"net/mail"
	"strings"
	"uuid"

	"github.com/samnart1/albergo/internal/domain/shared"
)

type Guest struct {
	ID       uuid.UUID
	Email    string
	FullName string
	Phone    string
}

func NewGuest(email, fullName, phone string) (Guest, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil {
		return Guest{}, shared.Invalid("invalid_email", "email %q is not a valid address", email)
	}

	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return Guest{}, shared.Invalid("invalid_guest_name", "guest name is required")
	}

	return Guest{
		ID:       uuid.New(),
		Email:    strings.ToLower(addr.Address),
		FullName: fullName,
		Phone:    strings.TrimSpace(phone),
	}, nil
}
