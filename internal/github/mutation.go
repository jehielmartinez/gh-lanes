package github

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/jehielmartinez/gh-lanes/internal/domain"
)

const updateBranchMutation = `mutation UpdatePullRequestBranch($input: UpdatePullRequestBranchInput!) {
  updatePullRequestBranch(input: $input) { pullRequest { id } }
}`

const markReadyMutation = `mutation MarkPullRequestReadyForReview($input: MarkPullRequestReadyForReviewInput!) {
  markPullRequestReadyForReview(input: $input) { pullRequest { id } }
}`

const convertToDraftMutation = `mutation ConvertPullRequestToDraft($input: ConvertPullRequestToDraftInput!) {
  convertPullRequestToDraft(input: $input) { pullRequest { id } }
}`

const mergeMutation = `mutation MergePullRequest($input: MergePullRequestInput!) {
  mergePullRequest(input: $input) { pullRequest { id } }
}`

const deleteRefMutation = `mutation DeleteRef($input: DeleteRefInput!) {
  deleteRef(input: $input) { clientMutationId }
}`

const enableAutoMergeMutation = `mutation EnablePullRequestAutoMerge($input: EnablePullRequestAutoMergeInput!) {
  enablePullRequestAutoMerge(input: $input) { pullRequest { id } }
}`

const disableAutoMergeMutation = `mutation DisablePullRequestAutoMerge($input: DisablePullRequestAutoMergeInput!) {
  disablePullRequestAutoMerge(input: $input) { pullRequest { id } }
}`

// Merge merges the pull request into its base branch with the method given.
func (c *Client) Merge(ctx context.Context, id string, method domain.MergeMethod) error {
	return c.mutate(ctx, mergeMutation, map[string]any{"pullRequestId": id, "mergeMethod": string(method)})
}

// DeleteBranch deletes the branch with the ref node ID given.
func (c *Client) DeleteBranch(ctx context.Context, refID string) error {
	return c.mutate(ctx, deleteRefMutation, map[string]any{"refId": refID})
}

// EnableAutoMerge has GitHub merge the pull request with the method given once
// its requirements are met.
func (c *Client) EnableAutoMerge(ctx context.Context, id string, method domain.MergeMethod) error {
	return c.mutate(ctx, enableAutoMergeMutation, map[string]any{"pullRequestId": id, "mergeMethod": string(method)})
}

// DisableAutoMerge cancels the pull request's pending auto-merge.
func (c *Client) DisableAutoMerge(ctx context.Context, id string) error {
	return c.mutate(ctx, disableAutoMergeMutation, map[string]any{"pullRequestId": id})
}

// UpdateBranch brings the pull request's head branch up to date with its own
// base branch, by merging the base in or by rebasing onto it.
func (c *Client) UpdateBranch(ctx context.Context, id string, method domain.UpdateMethod) error {
	input := map[string]any{"pullRequestId": id, "updateMethod": string(method)}
	return c.mutate(ctx, updateBranchMutation, input)
}

// MarkReadyForReview takes the pull request out of draft.
func (c *Client) MarkReadyForReview(ctx context.Context, id string) error {
	return c.mutate(ctx, markReadyMutation, map[string]any{"pullRequestId": id})
}

// ConvertToDraft turns the pull request back into a draft.
func (c *Client) ConvertToDraft(ctx context.Context, id string) error {
	return c.mutate(ctx, convertToDraftMutation, map[string]any{"pullRequestId": id})
}

// mutate runs a mutation whose response lanes doesn't need, reporting a
// refusal in GitHub's own words.
func (c *Client) mutate(ctx context.Context, mutation string, input map[string]any) error {
	var resp struct{}
	err := c.do(ctx, mutation, map[string]any{"input": input}, &resp)
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		return refusedError{err: gqlErr}
	}
	return err
}

// refusedError is GitHub declining a mutation. Its text is GitHub's messages
// alone, without the paths and prefix go-gh adds for debugging.
type refusedError struct{ err *api.GraphQLError }

func (e refusedError) Error() string {
	msgs := make([]string, 0, len(e.err.Errors))
	for _, item := range e.err.Errors {
		msgs = append(msgs, item.Message)
	}
	if len(msgs) == 0 {
		return fmt.Sprintf("GitHub refused the change: %v", e.err)
	}
	return strings.Join(msgs, "; ")
}

func (e refusedError) Unwrap() error { return e.err }
