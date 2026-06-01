package gest

import "net/http"

// HandlerOption configures runtime handler wrappers.
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	successStatus int
	emptyStatus   int
}

func newHandlerConfig(options ...HandlerOption) handlerConfig {
	config := handlerConfig{
		successStatus: http.StatusOK,
		emptyStatus:   http.StatusNoContent,
	}

	for _, option := range options {
		option(&config)
	}

	return config
}

// Status configures the success status used by typed handler wrappers.
func Status(code int) HandlerOption {
	return func(config *handlerConfig) {
		config.successStatus = code
		config.emptyStatus = code
	}
}
