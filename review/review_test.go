package review

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type mockClassifier struct {
	output *ClassifyOutput
	err    error
}

func (m *mockClassifier) ClassifyAll(_ context.Context, _ *ClassifyInput) (*ClassifyOutput, error) {
	return m.output, m.err
}

func (m *mockClassifier) Close() {}

func TestAnalyzeEmpty(t *testing.T) {
	data := &Data{}
	mock := &mockClassifier{output: &ClassifyOutput{}}
	results, err := Analyze(context.Background(), data, mock, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestAnalyzeReturnsEmptySliceNotNil(t *testing.T) {
	tests := []struct {
		name string
		data *Data
		mock *mockClassifier
	}{
		{
			name: "no threads or comments",
			data: &Data{},
			mock: &mockClassifier{output: &ClassifyOutput{}},
		},
		{
			name: "all resolved by classifier",
			data: &Data{
				Threads: []Thread{
					{
						ID:   "T1",
						Path: "main.go",
						Comments: []Comment{
							{ID: "C1", Body: "LGTM", Author: "alice", CreatedAt: time.Now(), URL: "https://example.com/1"},
						},
					},
				},
			},
			mock: &mockClassifier{
				output: &ClassifyOutput{
					Threads: []ClassifyOutputThread{
						{ThreadID: "T1", Category: "approval", IsResolved: true, Reason: "Approval"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := Analyze(context.Background(), tt.data, tt.mock, false)
			if err != nil {
				t.Fatal(err)
			}
			if results == nil {
				t.Fatal("expected non-nil empty slice, got nil")
			}
			b, err := json.Marshal(results)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != "[]" {
				t.Errorf("expected JSON output to be [], got %s", string(b))
			}
		})
	}
}

func TestAnalyzeFiltersResolved(t *testing.T) {
	data := &Data{
		Threads: []Thread{
			{
				ID:   "T1",
				Path: "main.go",
				Comments: []Comment{
					{ID: "C1", Body: "Fix this", Author: "alice", CreatedAt: time.Now(), URL: "https://example.com/1"},
				},
			},
			{
				ID:   "T2",
				Path: "main.go",
				Comments: []Comment{
					{ID: "C2", Body: "Looks good", Author: "bob", CreatedAt: time.Now(), URL: "https://example.com/2"},
				},
			},
		},
		PRComments: []Comment{
			{ID: "PC1", Body: "Overall feedback", Author: "carol", CreatedAt: time.Now(), URL: "https://example.com/3"},
		},
	}

	mock := &mockClassifier{
		output: &ClassifyOutput{
			Threads: []ClassifyOutputThread{
				{ThreadID: "T1", Category: "suggestion", IsResolved: false, Reason: "Not addressed"},
				{ThreadID: "T2", Category: "approval", IsResolved: true, Reason: "Approval comment"},
			},
			PRComments: []ClassifyOutputPRComment{
				{ID: "PC1", Category: "suggestion", IsResolved: false, Reason: "No follow-up"},
			},
		},
	}

	results, err := Analyze(context.Background(), data, mock, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 unresolved results, got %d", len(results))
	}

	// With showAll=true, should return all 3.
	results, err = Analyze(context.Background(), data, mock, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 total results, got %d", len(results))
	}
}

func TestAnalyzeGitHubResolvedOverrides(t *testing.T) {
	data := &Data{
		Threads: []Thread{
			{
				ID:         "T1",
				IsResolved: true,
				Path:       "main.go",
				Comments: []Comment{
					{ID: "C1", Body: "Fix this", Author: "alice", CreatedAt: time.Now(), URL: "https://example.com/1"},
				},
			},
		},
	}

	mock := &mockClassifier{
		output: &ClassifyOutput{
			Threads: []ClassifyOutputThread{
				{ThreadID: "T1", Category: "suggestion", IsResolved: false, Reason: "Not addressed per content"},
			},
		},
	}

	results, err := Analyze(context.Background(), data, mock, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (GitHub resolved overrides), got %d", len(results))
	}

	// With showAll=true, the thread is still reported, but as resolved on GitHub
	// rather than with the classifier result.
	results, err = Analyze(context.Background(), data, mock, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result with showAll, got %d", len(results))
	}
	if !results[0].Resolved {
		t.Error("expected resolved to be true")
	}
	if results[0].Category != "informational" {
		t.Errorf("expected category informational, got %s", results[0].Category)
	}
	if results[0].Reason != resolvedOnGitHubReason {
		t.Errorf("expected reason %q, got %q", resolvedOnGitHubReason, results[0].Reason)
	}
}

func TestAnalyzeUnclassifiedDefaultsToResolvedInformational(t *testing.T) {
	data := &Data{
		Threads: []Thread{
			{
				ID:   "T1",
				Path: "main.go",
				Comments: []Comment{
					{ID: "C1", Body: "Some comment", Author: "alice", CreatedAt: time.Now(), URL: "https://example.com/1"},
				},
			},
		},
		PRComments: []Comment{
			{ID: "PC1", Body: "FYI", Author: "bob", CreatedAt: time.Now(), URL: "https://example.com/2"},
		},
	}

	// Classifier returns empty results (no matching thread/comment IDs).
	mock := &mockClassifier{
		output: &ClassifyOutput{
			Threads:    []ClassifyOutputThread{},
			PRComments: []ClassifyOutputPRComment{},
		},
	}

	// With showAll=false, unclassified items should default to informational+resolved and be filtered out.
	results, err := Analyze(context.Background(), data, mock, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results (unclassified defaults to resolved informational), got %d", len(results))
		for _, r := range results {
			t.Logf("  category=%s resolved=%v", r.Category, r.Resolved)
		}
	}

	// With showAll=true, they should appear as informational+resolved.
	results, err = Analyze(context.Background(), data, mock, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results with showAll, got %d", len(results))
	}
	for _, r := range results {
		if r.Category != "informational" {
			t.Errorf("expected category informational, got %s", r.Category)
		}
		if !r.Resolved {
			t.Errorf("expected resolved=true for informational, got false")
		}
	}
}

func TestAnalyzePopulatesReplies(t *testing.T) {
	now := time.Now()
	data := &Data{
		Threads: []Thread{
			{
				ID:   "T1",
				Path: "main.go",
				Comments: []Comment{
					{ID: "C1", Body: "Why is this needed?", Author: "alice", CreatedAt: now, URL: "https://example.com/1"},
					{ID: "C2", Body: "It handles the edge case for empty input.", Author: "bob", CreatedAt: now.Add(time.Hour), URL: "https://example.com/2"},
				},
			},
		},
	}

	mock := &mockClassifier{
		output: &ClassifyOutput{
			Threads: []ClassifyOutputThread{
				{ThreadID: "T1", Category: "question", IsResolved: false, Reason: "Unanswered question"},
			},
		},
	}

	results, err := Analyze(context.Background(), data, mock, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.Author != "alice" {
		t.Errorf("expected author alice, got %s", r.Author)
	}
	if r.Body != "Why is this needed?" {
		t.Errorf("expected first comment body, got %s", r.Body)
	}
	if len(r.Replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(r.Replies))
	}
	if r.Replies[0].Author != "bob" {
		t.Errorf("expected reply author bob, got %s", r.Replies[0].Author)
	}
	if r.Replies[0].Body != "It handles the edge case for empty input." {
		t.Errorf("expected reply body, got %s", r.Replies[0].Body)
	}
}

func TestAnalyzeNoRepliesForSingleComment(t *testing.T) {
	data := &Data{
		Threads: []Thread{
			{
				ID:   "T1",
				Path: "main.go",
				Comments: []Comment{
					{ID: "C1", Body: "Fix this", Author: "alice", CreatedAt: time.Now(), URL: "https://example.com/1"},
				},
			},
		},
	}

	mock := &mockClassifier{
		output: &ClassifyOutput{
			Threads: []ClassifyOutputThread{
				{ThreadID: "T1", Category: "suggestion", IsResolved: false, Reason: "Not fixed"},
			},
		},
	}

	results, err := Analyze(context.Background(), data, mock, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Replies != nil {
		t.Errorf("expected nil replies for single comment, got %v", results[0].Replies)
	}
}

func TestBuildClassifyInput(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	line := 42
	data := &Data{
		Threads: []Thread{
			{
				ID:         "T1",
				IsResolved: false,
				Path:       "main.go",
				Line:       &line,
				Comments: []Comment{
					{ID: "C1", Body: "Fix this", Author: "alice", CreatedAt: now},
				},
			},
			{
				ID:         "T2",
				IsResolved: true,
				Path:       "main.go",
				Comments: []Comment{
					{ID: "C2", Body: "Already handled", Author: "alice", CreatedAt: now},
				},
			},
		},
		PRComments: []Comment{
			{ID: "PC1", Body: "Overall", Author: "bob", CreatedAt: now},
		},
	}

	suppressed := []SuppressedComment{
		{ID: "S1", Path: "main.go", Line: &line, Body: "Missing default case", Author: "copilot-pull-request-reviewer", CreatedAt: now},
		{ID: "S2", Path: "main.go", Body: "Stale", Author: "copilot-pull-request-reviewer", CreatedAt: now, IsOutdated: true},
	}

	input := buildClassifyInput(data, suppressed)

	if len(input.Threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(input.Threads))
	}
	if input.Threads[0].ThreadID != "T1" {
		t.Errorf("expected thread ID T1, got %s", input.Threads[0].ThreadID)
	}
	if input.Threads[0].Type != "inline" {
		t.Errorf("expected type inline, got %s", input.Threads[0].Type)
	}

	if len(input.PRComments) != 1 {
		t.Fatalf("expected 1 PR comment, got %d", len(input.PRComments))
	}
	if input.PRComments[0].ID != "PC1" {
		t.Errorf("expected PR comment ID PC1, got %s", input.PRComments[0].ID)
	}

	// Outdated suppressed comments are forced resolved, so they are not classified.
	if len(input.Suppressed) != 1 {
		t.Fatalf("expected 1 suppressed comment, got %d", len(input.Suppressed))
	}
	if input.Suppressed[0].ID != "S1" {
		t.Errorf("expected suppressed comment ID S1, got %s", input.Suppressed[0].ID)
	}
}
