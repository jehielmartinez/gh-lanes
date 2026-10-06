package ui_test

import (
	"testing"

	"github.com/jehielmartinez/gh-lanes/internal/github/githubtest"
)

func TestRequestsUseGHTokenAndDefaultHost(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport)
	h.waitForText("octo-org/sample-repo#7")

	req := transport.Requests()[0]
	if req.URL != "https://api.github.com/graphql" {
		t.Errorf("URL = %q, want the github.com GraphQL endpoint", req.URL)
	}
	if got := req.Header.Get("Authorization"); got != "token "+placeholderToken {
		t.Errorf("Authorization = %q, want the GH_TOKEN value", got)
	}
}

func TestRequestsGoToGHHost(t *testing.T) {
	transport := githubtest.New()
	transport.ReplyFixture(t, "SearchPullRequests", fixture("search_board.json"))
	h := newHarness(t, transport,
		withEnv("GH_HOST", "ghe.example.com"),
		withEnv("GH_ENTERPRISE_TOKEN", "placeholder-enterprise-token"),
	)
	h.waitForText("octo-org/sample-repo#7")

	req := transport.Requests()[0]
	if req.URL != "https://ghe.example.com/api/graphql" {
		t.Errorf("URL = %q, want the GH_HOST GraphQL endpoint", req.URL)
	}
	if got := req.Header.Get("Authorization"); got != "token placeholder-enterprise-token" {
		t.Errorf("Authorization = %q, want the enterprise token for GH_HOST", got)
	}
}
