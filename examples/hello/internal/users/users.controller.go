package users

import "github.com/r6m/gest"

// UserController handles example user routes.
// @Controller("/users")
// @Tag("Users")
type UserController struct {
	service *UserService
}

// NewUserController constructs the users controller.
func NewUserController(service *UserService) *UserController {
	return &UserController{service: service}
}

// FindUser returns one example user.
// @Get("/{id}")
// @Status(200)
// @Status(404)
// @Summary("Find user")
// @Description("Returns a user by ID.")
func (c *UserController) FindUser(ctx *gest.Context, request *FindUserRequest) (*UserResponse, error) {
	return c.service.FindUser(request)
}

// CreateUser creates an example user response.
// @Post("/")
// @Status(201)
// @Summary("Create user")
// @Description("Creates an example user from a JSON request body.")
func (c *UserController) CreateUser(ctx *gest.Context, request *CreateUserRequest) (*UserResponse, error) {
	response := c.service.CreateUser(request)
	return response, nil
}
