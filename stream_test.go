package gest

import (
	stdctx "context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContextStreamWritesHeadersStatusAndFlushes(t *testing.T) {
	recorder := httptest.NewRecorder()
	context := NewContext(recorder, httptest.NewRequest(http.MethodGet, "/export", nil))

	err := context.Stream(http.StatusAccepted, "text/csv", func(stream *Stream) error {
		if err := stream.WriteString("id,name\n"); err != nil {
			return err
		}
		return stream.Flush()
	})
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	if got := context.ResponseStatus(); got != http.StatusAccepted {
		t.Fatalf("ResponseStatus = %d, want %d", got, http.StatusAccepted)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/csv" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/csv")
	}
	if got := recorder.Body.String(); got != "id,name\n" {
		t.Fatalf("body = %q, want CSV row", got)
	}
	if !recorder.Flushed {
		t.Fatal("recorder.Flushed = false, want true")
	}
}

func TestContextStreamReturnsHandlerError(t *testing.T) {
	want := errors.New("stream failed")
	context := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/export", nil))

	err := context.Stream(http.StatusOK, "text/plain", func(stream *Stream) error {
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("Stream error = %v, want %v", err, want)
	}
}

func TestContextSSESetsHeadersTracksOKSendsJSONAndComments(t *testing.T) {
	recorder := httptest.NewRecorder()
	context := NewContext(recorder, httptest.NewRequest(http.MethodGet, "/events", nil))

	err := context.SSE(func(events *SSE) error {
		if got := context.ResponseStatus(); got != http.StatusOK {
			t.Fatalf("ResponseStatus in handler = %d, want %d", got, http.StatusOK)
		}
		if err := events.Send("user.updated", struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{ID: "u-1", Name: "Ada"}); err != nil {
			return err
		}
		return events.Comment("heartbeat")
	})
	if err != nil {
		t.Fatalf("SSE returned error: %v", err)
	}

	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want %q", got, "text/event-stream")
	}
	if got := context.ResponseStatus(); got != http.StatusOK {
		t.Fatalf("ResponseStatus = %d, want %d", got, http.StatusOK)
	}
	want := "event: user.updated\n" +
		"data: {\"id\":\"u-1\",\"name\":\"Ada\"}\n\n" +
		": heartbeat\n\n"
	if got := recorder.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if !recorder.Flushed {
		t.Fatal("recorder.Flushed = false, want true")
	}
}

func TestContextSSESendWithoutEventFormatsDataOnly(t *testing.T) {
	recorder := httptest.NewRecorder()
	context := NewContext(recorder, httptest.NewRequest(http.MethodGet, "/events", nil))

	err := context.SSE(func(events *SSE) error {
		return events.Send("", "hello")
	})
	if err != nil {
		t.Fatalf("SSE returned error: %v", err)
	}

	if got, want := recorder.Body.String(), "data: \"hello\"\n\n"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestContextSSEReturnsJSONEncodingError(t *testing.T) {
	context := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/events", nil))

	err := context.SSE(func(events *SSE) error {
		return events.Send("bad", func() {})
	})

	if err == nil {
		t.Fatal("SSE returned nil error, want JSON encoding error")
	}
}

func TestContextSSERespectsRequestCancellation(t *testing.T) {
	requestContext, cancel := stdctx.WithCancel(stdctx.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/events", nil).WithContext(requestContext)
	gestContext := NewContext(httptest.NewRecorder(), request)

	err := gestContext.SSE(func(events *SSE) error {
		return events.Send("ping", map[string]string{"ok": "true"})
	})

	if !errors.Is(err, stdctx.Canceled) {
		t.Fatalf("SSE error = %v, want stdctx.Canceled", err)
	}
}

func TestContextSSEReturnsHandlerError(t *testing.T) {
	want := errors.New("event source failed")
	context := NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/events", nil))

	err := context.SSE(func(events *SSE) error {
		return want
	})

	if !errors.Is(err, want) {
		t.Fatalf("SSE error = %v, want %v", err, want)
	}
}
