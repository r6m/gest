package users

// FindUserRequest demonstrates path and query DTO binding.
type FindUserRequest struct {
	ID     string `param:"id" validate:"required"`
	Expand bool   `query:"expand" default:"false"`
}

// CreateUserRequest demonstrates JSON body binding.
type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserResponse is returned by the users controller.
type UserResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Details string `json:"details,omitempty"`
}
