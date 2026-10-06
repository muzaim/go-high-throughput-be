package model

import "errors"

var (
	ErrItemNotFound                = errors.New("item does not exist")
	ErrInsufficientStock           = errors.New("insufficient inventory")
	ErrReservationNotFound         = errors.New("reservation does not exist")
	ErrReservationAlreadyConfirmed = errors.New("reservation already confirmed")
	ErrReservationExpired          = errors.New("reservation already expired")
	ErrInvalidQuantity             = errors.New("quantity must be greater than zero")
	ErrInvalidID                   = errors.New("invalid ID format")
)

type APIError struct {
	StatusCode int    `json:"-"`
	Status     string `json:"status"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    any    `json:"details"`
}

func (e *APIError) Error() string {
	return e.Message
}
