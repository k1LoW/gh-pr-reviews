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

// copilotOverviewV2Body mirrors the v2 Copilot review overview. "Resolved since
// last review" lists inline threads and must be ignored, while each
// "Previously missed" finding is its own nested <details> whose path carries
// zero-width spaces after every slash.
const copilotOverviewV2Body = "<!-- ccr-overview-v2 -->\n" +
	"\n" +
	"## Copilot review overview\n" +
	"\n" +
	"### 🔵 Needs a closer look\n" +
	"\n" +
	"**Findings:** None\n" +
	"\n" +
	"<details>\n" +
	"<summary><strong>Resolved since last review (1)</strong></summary>\n" +
	"\n" +
	"- <picture><img src=\"high.png\" alt=\"High severity\"></picture> [Timed-out preparation leaves backend running](#discussion_r4205434705)\n" +
	"</details>\n" +
	"\n" +
	"<details>\n" +
	"<summary><strong>Previously missed (2)</strong></summary>\n" +
	"\n" +
	"In code that hasn't changed since last review\n" +
	"\n" +
	"<details>\n" +
	"<summary><picture><source media=\"(prefers-color-scheme: dark)\" srcset=\"medium-dark.svg\"><img src=\"medium.png\" alt=\"Medium severity\"></picture> Use describeError to preserve actionable error descriptions</summary>\n" +
	"\n" +
	"`Sources/\u200bvo/\u200bTranslationWorker.swift:92`\n" +
	"\n" +
	"Use the repository's `describeError(_:)` helper here.\n" +
	"\n" +
	"This issue also appears on line 181 of the same file.\n" +
	"</details>\n" +
	"\n" +
	"<details>\n" +
	"<summary><picture><img src=\"low.png\" alt=\"Low severity\"></picture> Avoid colon as a sentence connector</summary>\n" +
	"\n" +
	"`README.md:35`\n" +
	"\n" +
	"The repository prose rule prohibits `:` as a sentence connector.\n" +
	"</details>\n" +
	"</details>\n" +
	"\n" +
	"<details>\n" +
	"<summary><strong>What changed in this PR</strong></summary>\n" +
	"\n" +
	"`main.go:1`\n" +
	"</details>\n"

func TestParsePreviouslyMissedSection(t *testing.T) {
	got := parsePreviouslyMissedSection(copilotOverviewV2Body)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got[0].path != "Sources/vo/TranslationWorker.swift" {
		t.Errorf("zero-width spaces not stripped from path: %q", got[0].path)
	}
	if got[0].line == nil || *got[0].line != 92 {
		t.Errorf("unexpected line: %v", got[0].line)
	}
	if !strings.HasPrefix(got[0].body, "Use describeError to preserve actionable error descriptions\n\n") {
		t.Errorf("title not leading the body: %q", got[0].body)
	}
	if strings.Contains(got[0].body, "<") {
		t.Errorf("HTML leaked into body: %q", got[0].body)
	}
	if !strings.Contains(got[0].body, "also appears on line 181") {
		t.Errorf("trailing paragraph dropped: %q", got[0].body)
	}
	if got[0].snippet != "" {
		t.Errorf("expected no snippet, got %q", got[0].snippet)
	}
	if got[1].path != "README.md" || got[1].line == nil || *got[1].line != 35 {
		t.Errorf("unexpected second entry location: %s:%v", got[1].path, got[1].line)
	}

	if got := parsePreviouslyMissedSection(copilotReviewBody); len(got) != 0 {
		t.Errorf("expected no entries from a body without the section, got %d", len(got))
	}
}

func TestExtractSuppressedCommentsPreviouslyMissed(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	other := "<details>\n<summary><strong>Previously missed (1)</strong></summary>\n\n" +
		"<details>\n<summary>Unwrap Collate in isValue</summary>\n\n`sharedtx.go:316`\n\nBody.\n</details>\n</details>\n"
	// Re-lists only the README.md finding from R_old.
	newest := "<details>\n<summary><strong>Previously missed (1)</strong></summary>\n\n" +
		"<details>\n<summary>Avoid colon</summary>\n\n`README.md:35`\n\nStill there.\n</details>\n</details>\n"
	reviews := []SubmittedReview{
		{ID: "R_old", Author: "copilot-pull-request-reviewer", SubmittedAt: older, Body: copilotOverviewV2Body},
		// A later review without the same findings must not supersede them,
		// because Copilot does not re-list previously missed findings.
		{ID: "R_mid", Author: "copilot-pull-request-reviewer", SubmittedAt: older.Add(time.Minute), Body: other},
		{ID: "R_new", Author: "copilot-pull-request-reviewer", SubmittedAt: newer, Body: newest},
	}

	got := ExtractSuppressedComments(reviews)
	byID := map[string]SuppressedComment{}
	for _, s := range got {
		byID[s.ID] = s
	}
	if len(byID) != 4 {
		t.Fatalf("expected 4 entries, got %d: %v", len(byID), byID)
	}
	if byID["R_old#previously-missed-0"].IsOutdated {
		t.Error("expected a finding not re-listed later to stay active")
	}
	if byID["R_mid#previously-missed-0"].IsOutdated {
		t.Error("expected the middle review's finding to stay active")
	}
	if s := byID["R_old#previously-missed-1"]; !s.IsOutdated || s.OutdatedReason != reportedAgainReason {
		t.Errorf("expected the duplicate in the older review to be outdated, got %v %q", s.IsOutdated, s.OutdatedReason)
	}
	if byID["R_new#previously-missed-0"].IsOutdated {
		t.Error("expected the newest occurrence to stay active")
	}
}
