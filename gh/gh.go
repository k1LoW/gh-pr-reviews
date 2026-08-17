package gh

import (
	"context"
	"fmt"
	"time"

	"github.com/k1LoW/gh-pr-reviews/review"
	"github.com/k1LoW/go-github-client/v79/factory"
	"github.com/shurcooL/githubv4"
)

// Client is a GitHub GraphQL API client for fetching PR review data.
type Client struct {
	v4 *githubv4.Client
}

// New creates a new Client.
func New() (*Client, error) {
	ghClient, err := factory.NewGithubClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}
	v4Client := githubv4.NewClient(ghClient.Client())
	return &Client{v4: v4Client}, nil
}

type threadCommentNode struct {
	ID         string
	DatabaseID int64
	Body       string
	Author     struct{ Login string }
	CreatedAt  time.Time
	URL        string `graphql:"url"`
	DiffHunk   string
	Commit     struct {
		Oid string
	}
}

type threadCommentConnection struct {
	Nodes    []threadCommentNode
	PageInfo struct {
		HasNextPage bool
		EndCursor   githubv4.String
	}
}

type reviewThreadsQuery struct {
	Repository struct {
		PullRequest struct {
			ReviewThreads struct {
				Nodes []struct {
					ID         string
					IsResolved bool
					IsOutdated bool
					Path       string
					Line       *int
					Comments   threadCommentConnection `graphql:"comments(first: 100)"`
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
			} `graphql:"reviewThreads(first: 100, after: $threadCursor)"`
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// threadCommentsQuery pages through a single thread's comments. The thread list
// query can only ask for a fixed page of comments per thread, so threads with
// more replies than that need to be topped up one by one via their node ID.
type threadCommentsQuery struct {
	Node struct {
		PullRequestReviewThread struct {
			Comments threadCommentConnection `graphql:"comments(first: 100, after: $commentCursor)"`
		} `graphql:"... on PullRequestReviewThread"`
	} `graphql:"node(id: $threadId)"`
}

type prCommentsQuery struct {
	Repository struct {
		PullRequest struct {
			Comments struct {
				Nodes []struct {
					ID        string
					Body      string
					Author    struct{ Login string }
					CreatedAt time.Time
					URL       string `graphql:"url"`
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
			} `graphql:"comments(first: 100, after: $commentCursor)"`
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// submittedReviewStates excludes PENDING so unsubmitted drafts never reach the
// suppressed comment extraction.
var submittedReviewStates = []githubv4.PullRequestReviewState{
	githubv4.PullRequestReviewStateApproved,
	githubv4.PullRequestReviewStateChangesRequested,
	githubv4.PullRequestReviewStateCommented,
	githubv4.PullRequestReviewStateDismissed,
}

type reviewsQuery struct {
	Repository struct {
		PullRequest struct {
			Reviews struct {
				Nodes []struct {
					ID          string
					Body        string
					Author      struct{ Login string }
					SubmittedAt *time.Time
					URL         string `graphql:"url"`
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   githubv4.String
				}
			} `graphql:"reviews(first: 100, after: $reviewCursor, states: $reviewStates)"`
		} `graphql:"pullRequest(number: $number)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// FetchReviews fetches all review threads, PR comments and review summary bodies
// for the given pull request.
func (c *Client) FetchReviews(ctx context.Context, owner, repo string, number int) (*review.Data, error) {
	data := &review.Data{}

	// Fetch review threads with pagination.
	var threadCursor *githubv4.String
	for {
		var q reviewThreadsQuery
		variables := map[string]any{
			"owner":        githubv4.String(owner),
			"repo":         githubv4.String(repo),
			"number":       githubv4.Int(int32(number)), //nolint:gosec
			"threadCursor": threadCursor,
		}
		if err := c.v4.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("failed to fetch review threads: %w", err)
		}
		for _, node := range q.Repository.PullRequest.ReviewThreads.Nodes {
			thread := review.Thread{
				ID:         node.ID,
				IsResolved: node.IsResolved,
				IsOutdated: node.IsOutdated,
				Path:       node.Path,
				Line:       node.Line,
			}
			for _, cm := range node.Comments.Nodes {
				thread.Comments = append(thread.Comments, toComment(cm))
			}
			if node.Comments.PageInfo.HasNextPage {
				rest, err := c.fetchRemainingThreadComments(ctx, node.ID, node.Comments.PageInfo.EndCursor)
				if err != nil {
					return nil, err
				}
				thread.Comments = append(thread.Comments, rest...)
			}
			data.Threads = append(data.Threads, thread)
		}
		if !q.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage {
			break
		}
		cursor := q.Repository.PullRequest.ReviewThreads.PageInfo.EndCursor
		threadCursor = &cursor
	}

	// Fetch PR-level comments with pagination.
	var commentCursor *githubv4.String
	for {
		var q prCommentsQuery
		variables := map[string]any{
			"owner":         githubv4.String(owner),
			"repo":          githubv4.String(repo),
			"number":        githubv4.Int(int32(number)), //nolint:gosec
			"commentCursor": commentCursor,
		}
		if err := c.v4.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("failed to fetch PR comments: %w", err)
		}
		for _, node := range q.Repository.PullRequest.Comments.Nodes {
			data.PRComments = append(data.PRComments, review.Comment{
				ID:        node.ID,
				Body:      node.Body,
				Author:    node.Author.Login,
				CreatedAt: node.CreatedAt,
				URL:       node.URL,
			})
		}
		if !q.Repository.PullRequest.Comments.PageInfo.HasNextPage {
			break
		}
		cursor := q.Repository.PullRequest.Comments.PageInfo.EndCursor
		commentCursor = &cursor
	}

	// Fetch review summary bodies with pagination.
	var reviewCursor *githubv4.String
	for {
		var q reviewsQuery
		variables := map[string]any{
			"owner":        githubv4.String(owner),
			"repo":         githubv4.String(repo),
			"number":       githubv4.Int(int32(number)), //nolint:gosec
			"reviewCursor": reviewCursor,
			"reviewStates": submittedReviewStates,
		}
		if err := c.v4.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("failed to fetch reviews: %w", err)
		}
		for _, node := range q.Repository.PullRequest.Reviews.Nodes {
			// Reviews submitted with inline comments only carry no summary body.
			if node.Body == "" {
				continue
			}
			// A nil submittedAt means the review was never submitted. Recording it
			// with a zero timestamp would make it sort as the oldest review, which
			// decides which suppressed comments count as superseded.
			if node.SubmittedAt == nil {
				continue
			}
			data.Reviews = append(data.Reviews, review.SubmittedReview{
				ID:          node.ID,
				Body:        node.Body,
				Author:      node.Author.Login,
				SubmittedAt: *node.SubmittedAt,
				URL:         node.URL,
			})
		}
		if !q.Repository.PullRequest.Reviews.PageInfo.HasNextPage {
			break
		}
		cursor := q.Repository.PullRequest.Reviews.PageInfo.EndCursor
		reviewCursor = &cursor
	}

	return data, nil
}

// fetchRemainingThreadComments pages through the comments of a single review
// thread, starting after cursor.
func (c *Client) fetchRemainingThreadComments(ctx context.Context, threadID string, cursor githubv4.String) ([]review.Comment, error) {
	var comments []review.Comment
	commentCursor := &cursor
	for {
		var q threadCommentsQuery
		variables := map[string]any{
			"threadId":      githubv4.ID(threadID),
			"commentCursor": commentCursor,
		}
		if err := c.v4.Query(ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("failed to fetch comments of review thread %s: %w", threadID, err)
		}
		conn := q.Node.PullRequestReviewThread.Comments
		for _, cm := range conn.Nodes {
			comments = append(comments, toComment(cm))
		}
		if !conn.PageInfo.HasNextPage {
			return comments, nil
		}
		next := conn.PageInfo.EndCursor
		commentCursor = &next
	}
}

func toComment(n threadCommentNode) review.Comment {
	return review.Comment{
		ID:         n.ID,
		DatabaseID: n.DatabaseID,
		Body:       n.Body,
		Author:     n.Author.Login,
		CreatedAt:  n.CreatedAt,
		URL:        n.URL,
		DiffHunk:   n.DiffHunk,
		CommitID:   n.Commit.Oid,
	}
}
