package session

import "github.com/r6m/gest"

// Controller handles session routes for the Phase 7 optional modules fixture.
// @Controller("/sessions")
// @Tag("Sessions")
type Controller struct {
	service *Service
}

func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

// Create creates a signed session token.
// @Post("/")
// @Status(201)
// @Summary("Create session")
// @Description("Creates a signed JWT for a validated subject.")
func (c *Controller) Create(ctx *gest.Context, request *CreateSessionRequest) (*CreateSessionResponse, error) {
	return c.service.Create(request)
}
