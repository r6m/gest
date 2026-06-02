package session

import (
	"log/slog"

	"github.com/r6m/gest/examples/phase7/internal/settings"
	jwtmodule "github.com/r6m/gest/modules/jwt"
)

type Service struct {
	config *settings.AppConfig
	logger *slog.Logger
	jwt    *jwtmodule.Service
}

func NewService(config *settings.AppConfig, logger *slog.Logger, jwt *jwtmodule.Service) *Service {
	return &Service{
		config: config,
		logger: logger,
		jwt:    jwt,
	}
}

func (s *Service) Create(request *CreateSessionRequest) (*CreateSessionResponse, error) {
	s.logger.Info("creating session", slog.String("subject", request.Subject))
	token, err := s.jwt.Sign(request.Subject, jwtmodule.Value("service", s.config.ServiceName))
	if err != nil {
		return nil, err
	}
	return &CreateSessionResponse{
		Service: s.config.ServiceName,
		Token:   token,
	}, nil
}
