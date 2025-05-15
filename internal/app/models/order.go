package models

import (
	"time"
)

type OrdersResponse struct {
	Number    string    `json:"number"`
	Status    string    `json:"status"`
	Accrual   float64   `json:"accrual,omitempty"`
	CreatedAt time.Time `json:"uploaded_at"`
}
