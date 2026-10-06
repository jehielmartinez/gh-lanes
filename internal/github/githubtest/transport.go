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

// Response is a canned reply for an operation. A zero Status means 200.
type Response struct {
	Status int
	Body   []byte
	// Err, when set, fails the request without a response, the way a dropped
	// connection or an unreachable host does.
	Err error
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
	matched  []*matchedReplies
	requests []Request
}

// matchedReplies are the replies for an operation sent with one variable set
// to one value.
type matchedReplies struct {
	operation, variable string
	value               any
	replies             []Response
}

// New returns a transport with no replies queued.
func New() *Transport {
	return &Transport{replies: map[string][]Response{}}
}

// ReplyWhen queues a response for the named operation sent with the variable
// set to value. A request that matches is answered from these replies, in
// order with the last one repeating, and never from those queued with Reply.
func (t *Transport) ReplyWhen(operation, variable string, value any, r Response) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if m := t.match(operation, map[string]any{variable: value}); m != nil {
		m.replies = append(m.replies, r)
		return
	}
	t.matched = append(t.matched, &matchedReplies{operation: operation, variable: variable, value: value, replies: []Response{r}})
}

// ReplyFixtureWhen queues, as ReplyWhen does, a 200 response whose body is the
// fixture file at path.
func (t *Transport) ReplyFixtureWhen(tb testing.TB, operation, variable string, value any, path string) {
	tb.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read fixture: %v", err)
	}
	t.ReplyWhen(operation, variable, value, Response{Body: body})
}

// HasRepliesWhen reports whether any reply is queued with ReplyWhen for the
// operation, variable and value.
func (t *Transport) HasRepliesWhen(operation, variable string, value any) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.match(operation, map[string]any{variable: value})
	return m != nil
}

// match is the first set of matched replies the request's operation and
// variables select, or nil.
func (t *Transport) match(operation string, variables map[string]any) *matchedReplies {
	for _, m := range t.matched {
		if v, ok := variables[m.variable]; m.operation == operation && ok && v == m.value {
			return m
		}
	}
	return nil
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
	t.Reply(operation, Response{Body: body})
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
	reply, ok := t.next(op, payload.Variables)
	t.mu.Unlock()

	if reply.Release != nil {
		select {
		case <-reply.Release:
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	if reply.Err != nil {
		return nil, reply.Err
	}
	if !ok {
		reply = Response{
			Body: fmt.Appendf(nil, `{"errors":[{"message":"githubtest: no reply queued for %s"}]}`, op),
		}
	}
	status := reply.Status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(reply.Body)),
		Request:    req,
	}, nil
}

func (t *Transport) next(op string, variables map[string]any) (Response, bool) {
	if m := t.match(op, variables); m != nil {
		reply := m.replies[0]
		if len(m.replies) > 1 {
			m.replies = m.replies[1:]
		}
		return reply, true
	}
	queue := t.replies[op]
	if len(queue) == 0 {
		return Response{}, false
	}
	if len(queue) > 1 {
		t.replies[op] = queue[1:]
	}
	return queue[0], true
}
