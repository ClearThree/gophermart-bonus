package models

type CreateWithdrawalRequest struct {
	Order  string  `json:"order"`
	Amount float64 `json:"sum"`
}
