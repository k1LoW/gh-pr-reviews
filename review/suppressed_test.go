package review

import (
	"context"
	"strings"
	"testing"
	"time"
)

// copilotReviewBody mirrors the shape of a real Copilot review summary: the
// suppressed entries live inside a <details> block, each one is a bold
// "path:line" header followed by a bullet and a fenced snippet, and the section
// ends with the "Files reviewed" footer.
const copilotReviewBody = "### 🟡 Not ready to approve\n" +
	"\n" +
	"There are a few correctness issues that should be addressed before approval.\n" +
	"\n" +
	"<details>\n" +
	"<summary>Review details</summary>\n" +
	"\n" +
	"### Files not reviewed (1)\n" +
	"\n" +
	"* **service/function/storage/mock/mock_storage.go**: Generated file\n" +
	"\n" +
	"### Suppressed comments (2)\n" +
	"\n" +
	"**service/function/controlplane/v1/service_internal.go:207**\n" +
	"* This switch has no default case. Treat unknown values like ResultUnknown.\n" +
	"\n" +
	"This issue also appears on line 317 of the same file.\n" +
	"```\n" +
	"\t\tcase runner.ResultSucceeded:\n" +
	"\t\t\t# not a heading\n" +
	"\t\t\tstatus = model.ExecutionStatusSuccess\n" +
	"```\n" +
	"**service/function/internal/job/runner/local.go:235**\n" +
	"* GetJobResult treats any os.Open error as \"job result not found\".\n" +
	"```\n" +
	"\tfile, err := os.Open(dir)\n" +
	"```\n" +
	"\n" +
	"- **Files reviewed:** 12/19 changed files\n" +
	"- **Comments generated:** 0 new\n" +
	"- **Review effort level:** Lite\n" +
	"</details>\n" +
	"\n" +
	"We're testing this review assessment.\n"

func TestParseSuppressedSection(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		check func(t *testing.T, got []suppressedEntry)
	}{
		{
			name: "real world copilot body",
			body: copilotReviewBody,
			check: func(t *testing.T, got []suppressedEntry) {
				t.Helper()
				if len(got) != 2 {
					t.Fatalf("expected 2 entries, got %d", len(got))
				}
				if got[0].path != "service/function/controlplane/v1/service_internal.go" {
					t.Errorf("unexpected path: %s", got[0].path)
				}
				if got[0].line == nil || *got[0].line != 207 {
					t.Errorf("unexpected line: %v", got[0].line)
				}
				if !strings.HasPrefix(got[0].body, "This switch has no default case.") {
					t.Errorf("bullet marker not stripped: %q", got[0].body)
				}
				if !strings.Contains(got[0].body, "also appears on line 317") {
					t.Errorf("trailing paragraph dropped: %q", got[0].body)
				}
				// A "#" line inside the fence must not end the section.
				if !strings.Contains(got[0].snippet, "case runner.ResultSucceeded:") {
					t.Errorf("unexpected snippet: %q", got[0].snippet)
				}
				if strings.Contains(got[0].snippet, "```") {
					t.Errorf("fence markers leaked into snippet: %q", got[0].snippet)
				}
				if got[1].path != "service/function/internal/job/runner/local.go" {
					t.Errorf("unexpected path: %s", got[1].path)
				}
				if got[1].line == nil || *got[1].line != 235 {
					t.Errorf("unexpected line: %v", got[1].line)
				}
			},
		},
		{
			name: "no suppressed section",
			body: "### 🟢 Looks good\n\n<details>\n<summary>Review details</summary>\n\n- **Files reviewed:** 3/3\n</details>\n",
			check: func(t *testing.T, got []suppressedEntry) {
				t.Helper()
				if len(got) != 0 {
					t.Fatalf("expected 0 entries, got %d", len(got))
				}
			},
		},
		{
			name: "entry without snippet",
			body: "### Suppressed comments (1)\n\n**main.go:10**\n* Prefer errors.Is here.\n</details>\n",
			check: func(t *testing.T, got []suppressedEntry) {
				t.Helper()
				if len(got) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(got))
				}
				if got[0].body != "Prefer errors.Is here." {
					t.Errorf("unexpected body: %q", got[0].body)
				}
				if got[0].snippet != "" {
					t.Errorf("expected no snippet, got %q", got[0].snippet)
				}
			},
		},
		{
			name: "crlf line endings",
			body: "### Suppressed comments (1)\r\n\r\n**main.go:10**\r\n* Prefer errors.Is here.\r\n",
			check: func(t *testing.T, got []suppressedEntry) {
				t.Helper()
				if len(got) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(got))
				}
				if got[0].path != "main.go" {
					t.Errorf("unexpected path: %q", got[0].path)
				}
			},
		},
		{
			name: "next heading ends the section",
			body: "### Suppressed comments (1)\n\n**main.go:10**\n* Kept.\n\n### Other section\n\n**other.go:20**\n* Dropped.\n",
			check: func(t *testing.T, got []suppressedEntry) {
				t.Helper()
				if len(got) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(got))
				}
				if got[0].body != "Kept." {
					t.Errorf("unexpected body: %q", got[0].body)
				}
			},
		},
		{
			name: "path containing a colon",
			body: "### Suppressed comments (1)\n\n**pkg/a:b/main.go:10**\n* Body.\n",
			check: func(t *testing.T, got []suppressedEntry) {
				t.Helper()
				if len(got) != 1 {
					t.Fatalf("expected 1 entry, got %d", len(got))
				}
				if got[0].path != "pkg/a:b/main.go" {
					t.Errorf("unexpected path: %q", got[0].path)
				}
				if got[0].line == nil || *got[0].line != 10 {
					t.Errorf("unexpected line: %v", got[0].line)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, parseSuppressedSection(tt.body))
		})
	}
}

func TestExtractSuppressedCommentsOnlyCopilot(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	reviews := []SubmittedReview{
		{ID: "R1", Author: "alice", SubmittedAt: now, Body: copilotReviewBody},
		{ID: "R2", Author: "copilot-pull-request-reviewer[bot]", SubmittedAt: now, Body: copilotReviewBody},
	}

	got := ExtractSuppressedComments(reviews)
	if len(got) != 2 {
		t.Fatalf("expected 2 suppressed comments, got %d", len(got))
	}
	for _, s := range got {
		if !strings.HasPrefix(s.ID, "R2#suppressed-") {
			t.Errorf("expected entries only from the Copilot review, got ID %q", s.ID)
		}
	}
}

func TestExtractSuppressedCommentsMarksOlderReviewsOutdated(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	body := "### Suppressed comments (1)\n\n**main.go:10**\n* Body.\n"
	reviews := []SubmittedReview{
		{ID: "R_new", Author: "copilot-pull-request-reviewer", SubmittedAt: newer, Body: body, URL: "https://example.com/new"},
		{ID: "R_old", Author: "copilot-pull-request-reviewer", SubmittedAt: older, Body: body, URL: "https://example.com/old"},
	}

	got := ExtractSuppressedComments(reviews)
	if len(got) != 2 {
		t.Fatalf("expected 2 suppressed comments, got %d", len(got))
	}
	outdated := map[string]bool{}
	for _, s := range got {
		outdated[s.ID] = s.IsOutdated
	}
	if outdated["R_new#suppressed-0"] {
		t.Error("expected the newest review's entry to not be outdated")
	}
	if !outdated["R_old#suppressed-0"] {
		t.Error("expected the older review's entry to be outdated")
	}
}

func TestAnalyzeUnclassifiedSuppressedStaysUnresolved(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	data := &Data{
		Reviews: []SubmittedReview{
			{ID: "R1", Author: "copilot-pull-request-reviewer", SubmittedAt: now, Body: "### Suppressed comments (1)\n\n**main.go:10**\n* Missing default case.\n"},
		},
	}
	// The classifier returned valid JSON but omitted the suppressed entry.
	mock := &mockClassifier{output: &ClassifyOutput{}}

	results, err := Analyze(context.Background(), data, mock, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected the unclassified suppressed comment to be reported, got %d results", len(results))
	}
	if results[0].Resolved {
		t.Error("expected an unclassified suppressed comment to stay unresolved")
	}
	if results[0].Reason != unclassifiedReason {
		t.Errorf("unexpected reason: %q", results[0].Reason)
	}
}

func TestAnalyzeSuppressedComments(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	body := "### Suppressed comments (1)\n\n**main.go:10**\n* Missing default case.\n```\nswitch v {\n```\n"
	data := &Data{
		Reviews: []SubmittedReview{
			{ID: "R_new", Author: "copilot-pull-request-reviewer", SubmittedAt: newer, Body: body, URL: "https://example.com/new"},
			{ID: "R_old", Author: "copilot-pull-request-reviewer", SubmittedAt: older, Body: body, URL: "https://example.com/old"},
		},
	}
	mock := &mockClassifier{
		output: &ClassifyOutput{
			Suppressed: []ClassifyOutputSuppressed{
				{ID: "R_new#suppressed-0", Category: "issue", IsResolved: false, Reason: "not addressed"},
			},
		},
	}

	results, err := Analyze(context.Background(), data, mock, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Type != "suppressed" {
		t.Errorf("expected type suppressed, got %s", r.Type)
	}
	if r.ThreadID != "" || r.CommentID != 0 {
		t.Errorf("expected no thread/comment ID, got %q/%d", r.ThreadID, r.CommentID)
	}
	if r.Path != "main.go" || r.Line == nil || *r.Line != 10 {
		t.Errorf("unexpected location: %s:%v", r.Path, r.Line)
	}
	if r.Category != "issue" || r.Resolved {
		t.Errorf("unexpected classification: %s resolved=%v", r.Category, r.Resolved)
	}
	if !strings.Contains(r.DiffHunk, "switch v {") {
		t.Errorf("expected the snippet in diff_hunk, got %q", r.DiffHunk)
	}
	if r.URL != "https://example.com/new" {
		t.Errorf("unexpected URL: %s", r.URL)
	}

	// With showAll, the superseded entry from the older review is included too.
	all, err := Analyze(context.Background(), data, mock, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 results with showAll, got %d", len(all))
	}
	if !all[1].Resolved || all[1].Reason != supersededReason {
		t.Errorf("expected the older entry to be resolved as superseded, got resolved=%v reason=%q", all[1].Resolved, all[1].Reason)
	}
}
