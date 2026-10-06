// Package githubtest provides the fake GraphQL transport tests inject into the
// GitHub layer. It replays canned responses per GraphQL operation and records
// every request it receives.
package githubtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"
)

// Request is one GraphQL request as it reached the transport.
type Request struct {
	URL       string
	Header    http.Header
	Operation string
	Query     string
	Variables map[string]any
}

// Response is a canned reply for an operation.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
	// Release, when set, holds the response back until it is closed, so a
	// test can see what the app shows while a request is in flight.
	Release <-chan struct{}
}

// Transport is an http.RoundTripper that answers GraphQL requests from canned
// responses. Each operation's replies are served in order, and the last one
// repeats.
type Transport struct {
	mu       sync.Mutex
	replies  map[string][]Response
	requests []Request
}

// New returns a transport with no replies queued.
func New() *Transport {
	return &Transport{replies: map[string][]Response{}}
}

// Reply queues a response for the named operation.
func (t *Transport) Reply(operation string, r Response) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.replies[operation] = append(t.replies[operation], r)
}

// ReplyFixture queues a 200 response whose body is the fixture file at path.
func (t *Transport) ReplyFixture(tb testing.TB, operation, path string) {
	tb.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read fixture: %v", err)
	}
	t.Reply(operation, Response{Status: http.StatusOK, Body: body})
}

// Requests returns every request received so far, oldest first.
func (t *Transport) Requests() []Request {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]Request(nil), t.requests...)
}

var operationName = regexp.MustCompile(`^\s*(?:query|mutation)\s+(\w+)`)

// RoundTrip records the request and answers it with the next reply queued for
// its operation, or a GraphQL error naming the operation if none is queued.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("githubtest: request body is not GraphQL JSON: %w", err)
	}
	op := ""
	if m := operationName.FindStringSubmatch(payload.Query); m != nil {
		op = m[1]
	}

	t.mu.Lock()
	t.requests = append(t.requests, Request{
		URL:       req.URL.String(),
		Header:    req.Header.Clone(),
		Operation: op,
		Query:     payload.Query,
		Variables: payload.Variables,
	})
	reply, ok := t.next(op)
	t.mu.Unlock()

	if reply.Release != nil {
		select {
		case <-reply.Release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	if !ok {
		reply = Response{
			Status: http.StatusOK,
			Body:   fmt.Appendf(nil, `{"errors":[{"message":"githubtest: no reply queued for %s"}]}`, op),
		}
	}
	header := reply.Header.Clone()
	if header == nil {
		header = http.Header{}
	}
	if header.Get("Content-Type") == "" {
		header.Set("Content-Type", "application/json")
	}
	return &http.Response{
		StatusCode: reply.Status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(reply.Body)),
		Request:    req,
	}, nil
}

func (t *Transport) next(op string) (Response, bool) {
	queue := t.replies[op]
	if len(queue) == 0 {
		return Response{}, false
	}
	if len(queue) > 1 {
		t.replies[op] = queue[1:]
	}
	return queue[0], true
}
