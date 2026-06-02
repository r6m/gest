package gest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Stream writes a streaming response while preserving access to the raw
// net/http request and response through the Context escape hatches.
func (c *Context) Stream(status int, contentType string, fn func(stream *Stream) error) error {
	if contentType != "" {
		c.response.Header().Set("Content-Type", contentType)
	}
	c.response.WriteHeader(status)
	return fn(&Stream{response: c.response})
}

// SSE writes a Server-Sent Events response.
func (c *Context) SSE(fn func(events *SSE) error) error {
	c.response.Header().Set("Content-Type", "text/event-stream")
	c.response.WriteHeader(http.StatusOK)
	return fn(&SSE{
		stream: &Stream{response: c.response},
		ctx:    c.request.Context(),
	})
}

// Stream is a small net/http-backed streaming writer.
type Stream struct {
	response http.ResponseWriter
}

// Write writes bytes to the stream.
func (s *Stream) Write(data []byte) error {
	_, err := s.response.Write(data)
	return err
}

// WriteString writes text to the stream.
func (s *Stream) WriteString(data string) error {
	_, err := s.response.Write([]byte(data))
	return err
}

// Flush flushes the stream when the underlying response writer supports it.
func (s *Stream) Flush() error {
	flusher, ok := s.response.(http.Flusher)
	if !ok {
		return errors.New("response writer does not support flushing")
	}
	flusher.Flush()
	return nil
}

// SSE writes Server-Sent Event frames.
type SSE struct {
	stream *Stream
	ctx    context.Context
}

// Send writes a named SSE event with JSON-encoded data and flushes it.
func (s *SSE) Send(event string, data any) error {
	if err := s.contextError(); err != nil {
		return err
	}

	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var builder strings.Builder
	if event != "" {
		builder.WriteString("event: ")
		builder.WriteString(event)
		builder.WriteByte('\n')
	}
	builder.WriteString("data: ")
	builder.Write(encoded)
	builder.WriteString("\n\n")

	if err := s.stream.WriteString(builder.String()); err != nil {
		return err
	}
	if err := s.stream.Flush(); err != nil {
		return err
	}
	return s.contextError()
}

// Comment writes an SSE comment frame and flushes it.
func (s *SSE) Comment(text string) error {
	if err := s.contextError(); err != nil {
		return err
	}

	var builder strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		builder.WriteString(": ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteByte('\n')

	if err := s.stream.WriteString(builder.String()); err != nil {
		return err
	}
	if err := s.stream.Flush(); err != nil {
		return err
	}
	return s.contextError()
}

func (s *SSE) contextError() error {
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("sse request canceled: %w", s.ctx.Err())
	default:
		return nil
	}
}
