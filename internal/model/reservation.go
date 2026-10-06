package model

import "time"

type ReservationStatus string

const (
	StatusActive    ReservationStatus = "ACTIVE"
	StatusConfirmed ReservationStatus = "CONFIRMED"
	StatusExpired   ReservationStatus = "EXPIRED"
)

type Reservation struct {
	ID          string            `json:"reservation_id"`
	UserID      string            `json:"user_id"`
	ItemID      string            `json:"item_id"`
	Quantity    int               `json:"quantity"`
	Status      ReservationStatus `json:"status"`
	ExpiresAt   time.Time         `json:"expires_at"`
	ConfirmedAt *time.Time        `json:"confirmed_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type ReserveRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	ItemID   string `json:"item_id" binding:"required"`
	Quantity int    `json:"quantity" binding:"required,gt=0"`
}

type ReserveResponse struct {
	Status        string    `json:"status"`
	ReservationID string    `json:"reservation_id"`
	ItemID        string    `json:"item_id"`
	Quantity      int       `json:"quantity"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type ConfirmRequest struct {
	ReservationID string `json:"reservation_id" binding:"required"`
}

type ConfirmResponse struct {
	Status        string    `json:"status"`
	ReservationID string    `json:"reservation_id"`
	ConfirmedAt   time.Time `json:"confirmed_at"`
}
