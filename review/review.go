package review

import (
	"context"
	"fmt"
	"time"
)

// Comment represents a single review comment.
type Comment struct {
	ID         string    `json:"id"`
	DatabaseID int64     `json:"database_id"`
	Body       string    `json:"body"`
	Author     string    `json:"author"`
	CreatedAt  time.Time `json:"created_at"`
	URL        string    `json:"url"`
	CommitID   string    `json:"commit_id,omitempty"`
}

// Thread represents an inline review thread.
type Thread struct {
	ID         string    `json:"id"`
	IsResolved bool      `json:"is_resolved"`
	IsOutdated bool      `json:"is_outdated"`
	Path       string    `json:"path"`
	Line       *int      `json:"line,omitempty"`
	Comments   []Comment `json:"comments"`
}

// Data holds all review data for a PR.
type Data struct {
	Threads    []Thread  `json:"threads"`
	PRComments []Comment `json:"pr_comments"`
}

// ClassifyInputThread is a thread entry sent to the classifier.
type ClassifyInputThread struct {
	ThreadID           string                 `json:"thread_id"`
	Type               string                 `json:"type"`
	Path               string                 `json:"path,omitempty"`
	Line               *int                   `json:"line,omitempty"`
	IsResolvedOnGitHub bool                   `json:"is_resolved_on_github"`
	Comments           []ClassifyInputComment `json:"comments"`
}

// ClassifyInputComment is a comment entry sent to the classifier.
type ClassifyInputComment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// ClassifyInputPRComment is a PR-level comment entry sent to the classifier.
type ClassifyInputPRComment struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// ClassifyInput is the full input sent to the classifier.
type ClassifyInput struct {
	Threads    []ClassifyInputThread    `json:"threads"`
	PRComments []ClassifyInputPRComment `json:"pr_comments"`
}

// ClassifyOutputThread is a classified thread result.
type ClassifyOutputThread struct {
	ThreadID   string `json:"thread_id"`
	Category   string `json:"category"`
	IsResolved bool   `json:"is_resolved"`
	Reason     string `json:"reason"`
}

// ClassifyOutputPRComment is a classified PR comment result.
type ClassifyOutputPRComment struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	IsResolved bool   `json:"is_resolved"`
	Reason     string `json:"reason"`
}

// ClassifyOutput is the full output from the classifier.
type ClassifyOutput struct {
	Threads    []ClassifyOutputThread    `json:"threads"`
	PRComments []ClassifyOutputPRComment `json:"pr_comments"`
}

// CommentClassifier classifies review comments.
type CommentClassifier interface {
	ClassifyAll(ctx context.Context, input *ClassifyInput) (*ClassifyOutput, error)
	Close()
}

// UnresolvedComment is the JSON output structure for CLI results.
type UnresolvedComment struct {
	ThreadID  string `json:"thread_id,omitempty"`
	CommentID int64  `json:"comment_id"`
	Type      string `json:"type"`
	Path      string `json:"path,omitempty"`
	Line      *int   `json:"line,omitempty"`
	CommitID  string `json:"commit_id,omitempty"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	Category  string `json:"category"`
	Resolved  bool   `json:"resolved"`
	Reason    string `json:"reason"`
}

// Analyze classifies and filters review comments, returning unresolved ones (or all if showAll is true).
func Analyze(ctx context.Context, data *Data, classifier CommentClassifier, showAll bool) ([]UnresolvedComment, error) {
	if len(data.Threads) == 0 && len(data.PRComments) == 0 {
		return []UnresolvedComment{}, nil
	}

	input := buildClassifyInput(data)

	output, err := classifier.ClassifyAll(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to classify comments: %w", err)
	}

	return buildResults(data, output, showAll), nil
}

func buildClassifyInput(data *Data) *ClassifyInput {
	input := &ClassifyInput{}

	for _, t := range data.Threads {
		ct := ClassifyInputThread{
			ThreadID:           t.ID,
			Type:               "inline",
			Path:               t.Path,
			Line:               t.Line,
			IsResolvedOnGitHub: t.IsResolved,
		}
		for _, c := range t.Comments {
			ct.Comments = append(ct.Comments, ClassifyInputComment{
				Author:    c.Author,
				Body:      c.Body,
				CreatedAt: c.CreatedAt.Format(time.RFC3339),
			})
		}
		input.Threads = append(input.Threads, ct)
	}

	for _, c := range data.PRComments {
		input.PRComments = append(input.PRComments, ClassifyInputPRComment{
			ID:        c.ID,
			Author:    c.Author,
			Body:      c.Body,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		})
	}

	return input
}

func buildResults(data *Data, output *ClassifyOutput, showAll bool) []UnresolvedComment {
	var results []UnresolvedComment

	threadMap := make(map[string]*ClassifyOutputThread, len(output.Threads))
	for i := range output.Threads {
		threadMap[output.Threads[i].ThreadID] = &output.Threads[i]
	}

	for _, t := range data.Threads {
		classified, ok := threadMap[t.ID]
		resolved := t.IsResolved
		category := "unknown"
		reason := ""
		if ok {
			category = classified.Category
			reason = classified.Reason
			if !resolved {
				resolved = classified.IsResolved
			}
		}

		if !showAll && resolved {
			continue
		}

		// Use the first comment as the representative.
		var author, body, url, commitID string
		var commentID int64
		if len(t.Comments) > 0 {
			author = t.Comments[0].Author
			body = t.Comments[0].Body
			url = t.Comments[0].URL
			commentID = t.Comments[0].DatabaseID
			commitID = t.Comments[0].CommitID
		}

		results = append(results, UnresolvedComment{
			ThreadID:  t.ID,
			CommentID: commentID,
			Type:      "thread",
			Path:      t.Path,
			Line:      t.Line,
			CommitID:  commitID,
			Author:    author,
			Body:      body,
			URL:       url,
			Category:  category,
			Resolved:  resolved,
			Reason:    reason,
		})
	}

	commentMap := make(map[string]*ClassifyOutputPRComment, len(output.PRComments))
	for i := range output.PRComments {
		commentMap[output.PRComments[i].ID] = &output.PRComments[i]
	}

	for _, c := range data.PRComments {
		classified, ok := commentMap[c.ID]
		resolved := false
		category := "unknown"
		reason := ""
		if ok {
			category = classified.Category
			resolved = classified.IsResolved
			reason = classified.Reason
		}

		if !showAll && resolved {
			continue
		}

		results = append(results, UnresolvedComment{
			CommentID: c.DatabaseID,
			Type:      "comment",
			Author:    c.Author,
			Body:      c.Body,
			URL:       c.URL,
			Category:  category,
			Resolved:  resolved,
			Reason:    reason,
		})
	}

	return results
}
