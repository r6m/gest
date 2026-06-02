package session

type CreateSessionRequest struct {
	Subject string `json:"subject" validate:"required"`
}

type CreateSessionResponse struct {
	Service string `json:"service"`
	Token   string `json:"token"`
}
