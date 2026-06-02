package users

import "github.com/r6m/gest"

// UserService is an in-memory service for the hello example.
type UserService struct {
	users map[string]UserResponse
}

// NewUserService constructs the users service.
func NewUserService() *UserService {
	return &UserService{
		users: map[string]UserResponse{
			"123": {
				ID:    "123",
				Name:  "Ada Lovelace",
				Email: "ada@example.test",
			},
		},
	}
}

// FindUser returns a user or a framework 404 error.
func (s *UserService) FindUser(request *FindUserRequest) (*UserResponse, error) {
	user, ok := s.users[request.ID]
	if !ok {
		return nil, gest.NotFound("user not found")
	}
	if request.Expand {
		user.Details = "Example user served from an in-memory provider."
	}
	return &user, nil
}

// CreateUser returns a deterministic user response for tests and docs.
func (s *UserService) CreateUser(request *CreateUserRequest) *UserResponse {
	return &UserResponse{
		ID:    "new",
		Name:  request.Name,
		Email: request.Email,
	}
}
