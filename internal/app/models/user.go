package models

type LoginPasswordRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type GetBalancesResponse struct {
	Current   float32 `json:"current"`
	Withdrawn float32 `json:"withdrawn"`
}
