package review

import (
	"context"
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
		},
		PRComments: []Comment{
			{ID: "PC1", Body: "Overall", Author: "bob", CreatedAt: now},
		},
	}

	input := buildClassifyInput(data)

	if len(input.Threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(input.Threads))
	}
	if input.Threads[0].ThreadID != "T1" {
		t.Errorf("expected thread ID T1, got %s", input.Threads[0].ThreadID)
	}
	if input.Threads[0].IsResolvedOnGitHub {
		t.Error("expected IsResolvedOnGitHub to be false")
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
}
