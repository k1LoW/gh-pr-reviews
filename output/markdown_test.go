package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/k1LoW/gh-pr-reviews/review"
	"github.com/muesli/termenv"
)

func newTestOutput() *termenv.Output {
	return termenv.NewOutput(new(bytes.Buffer), termenv.WithProfile(termenv.Ascii))
}

func TestRenderMarkdownEmpty(t *testing.T) {
	var buf bytes.Buffer
	RenderMarkdown(&buf, nil, newTestOutput(), 80)
	want := "No unresolved comments found.\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestRenderMarkdownEmptySlice(t *testing.T) {
	var buf bytes.Buffer
	RenderMarkdown(&buf, []review.UnresolvedComment{}, newTestOutput(), 80)
	want := "No unresolved comments found.\n"
	if buf.String() != want {
		t.Errorf("got %q, want %q", buf.String(), want)
	}
}

func TestRenderMarkdownGroupedThreads(t *testing.T) {
	line1 := 42
	line2 := 58
	results := []review.UnresolvedComment{
		{
			ThreadID: "t1",
			Type:     "thread",
			Path:     "src/main.go",
			Line:     &line1,
			Author:   "alice",
			Body:     "Fix this typo",
			URL:      "https://github.com/example/1",
			Category: "suggestion",
			Resolved: false,
		},
		{
			ThreadID: "t2",
			Type:     "thread",
			Path:     "src/main.go",
			Line:     &line2,
			Author:   "bob",
			Body:     "Handle nil input",
			URL:      "https://github.com/example/2",
			Category: "issue",
			Resolved: false,
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	// Verify file path header appears once.
	if count := strings.Count(out, "## src/main.go"); count != 1 {
		t.Errorf("expected file header once, got %d times", count)
	}

	// Verify both comments are present.
	if !strings.Contains(out, "suggestion") {
		t.Error("missing category 'suggestion'")
	}
	if !strings.Contains(out, "issue") {
		t.Error("missing category 'issue'")
	}
	if !strings.Contains(out, "@alice") {
		t.Error("missing author @alice")
	}
	if !strings.Contains(out, "@bob") {
		t.Error("missing author @bob")
	}
	if !strings.Contains(out, "L42") {
		t.Error("missing line number L42")
	}
	if !strings.Contains(out, "L58") {
		t.Error("missing line number L58")
	}

	// Verify separator between comments.
	if !strings.Contains(out, "---") {
		t.Error("missing separator between comments")
	}
}

func TestRenderMarkdownPRComments(t *testing.T) {
	results := []review.UnresolvedComment{
		{
			Type:     "comment",
			Author:   "carol",
			Body:     "Can we add tests?",
			URL:      "https://github.com/example/3",
			Category: "question",
			Resolved: false,
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	if !strings.Contains(out, "## PR Comments") {
		t.Error("missing PR Comments header")
	}
	if !strings.Contains(out, "question") {
		t.Error("missing category 'question'")
	}
	if !strings.Contains(out, "@carol") {
		t.Error("missing author @carol")
	}
}

func TestRenderMarkdownMixedTypes(t *testing.T) {
	line := 10
	results := []review.UnresolvedComment{
		{
			ThreadID: "t1",
			Type:     "thread",
			Path:     "lib/utils.go",
			Line:     &line,
			Author:   "alice",
			Body:     "Thread comment",
			URL:      "https://github.com/example/1",
			Category: "nitpick",
			Resolved: false,
		},
		{
			Type:     "comment",
			Author:   "bob",
			Body:     "PR comment",
			URL:      "https://github.com/example/2",
			Category: "question",
			Resolved: false,
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	if !strings.Contains(out, "## lib/utils.go") {
		t.Error("missing file path header")
	}
	if !strings.Contains(out, "## PR Comments") {
		t.Error("missing PR Comments header")
	}
}

func TestRenderMarkdownSuppressed(t *testing.T) {
	threadLine := 10
	suppressedLine := 42
	results := []review.UnresolvedComment{
		{
			ThreadID: "t1",
			Type:     "thread",
			Path:     "lib/utils.go",
			Line:     &threadLine,
			Author:   "alice",
			Body:     "Thread comment",
			URL:      "https://github.com/example/1",
			Category: "nitpick",
		},
		{
			Type:     "suppressed",
			Path:     "lib/utils.go",
			Line:     &suppressedLine,
			Author:   "copilot-pull-request-reviewer",
			Body:     "Suppressed finding",
			URL:      "https://github.com/example/pull/1#pullrequestreview-1",
			Category: "issue",
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	// Suppressed comments share the file group with inline threads on the same path.
	if n := strings.Count(out, "## lib/utils.go"); n != 1 {
		t.Errorf("expected 1 file path header, got %d", n)
	}
	if strings.Contains(out, "## PR Comments") {
		t.Error("suppressed comments must not be rendered under PR Comments")
	}
	if !strings.Contains(out, "[suppressed]") {
		t.Error("missing [suppressed] marker")
	}
	if strings.Count(out, "[suppressed]") != 1 {
		t.Error("the [suppressed] marker must only be applied to suppressed comments")
	}
	if !strings.Contains(out, "L42") {
		t.Error("missing suppressed comment line number")
	}
}

func TestRenderMarkdownResolvedStatus(t *testing.T) {
	results := []review.UnresolvedComment{
		{
			Type:     "comment",
			Author:   "alice",
			Body:     "Resolved comment",
			URL:      "https://github.com/example/1",
			Category: "suggestion",
			Resolved: true,
		},
		{
			Type:     "comment",
			Author:   "bob",
			Body:     "Unresolved comment",
			URL:      "https://github.com/example/2",
			Category: "issue",
			Resolved: false,
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	if !strings.Contains(out, "(resolved)") {
		t.Error("missing '(resolved)' status")
	}
	if !strings.Contains(out, "(unresolved)") {
		t.Error("missing '(unresolved)' status")
	}
}

func TestRenderMarkdownNoLine(t *testing.T) {
	results := []review.UnresolvedComment{
		{
			ThreadID: "t1",
			Type:     "thread",
			Path:     "README.md",
			Line:     nil,
			Author:   "alice",
			Body:     "Comment without line",
			URL:      "https://github.com/example/1",
			Category: "suggestion",
			Resolved: false,
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	// Should not contain "L" prefix for line number.
	if strings.Contains(out, "L0") {
		t.Error("should not show line number when nil")
	}

	// URL should still be present.
	if !strings.Contains(out, "https://github.com/example/1") {
		t.Error("missing URL")
	}
}

func TestRenderMarkdownReplies(t *testing.T) {
	line := 42
	results := []review.UnresolvedComment{
		{
			ThreadID: "t1",
			Type:     "thread",
			Path:     "src/main.go",
			Line:     &line,
			Author:   "alice",
			Body:     "Why is this needed?",
			URL:      "https://github.com/example/1",
			Category: "question",
			Resolved: false,
			Replies: []review.ReplyComment{
				{
					Author: "bob",
					Body:   "It handles the edge case for empty input.",
					URL:    "https://github.com/example/2",
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	if !strings.Contains(out, "@bob") {
		t.Error("missing reply author @bob")
	}
	if !strings.Contains(out, "edge case for empty input") {
		t.Error("missing reply body")
	}
	if !strings.Contains(out, ">") {
		t.Error("missing blockquote prefix for reply")
	}
}

func TestRenderMarkdownNoReplies(t *testing.T) {
	results := []review.UnresolvedComment{
		{
			Type:     "comment",
			Author:   "alice",
			Body:     "Just a comment",
			URL:      "https://github.com/example/1",
			Category: "suggestion",
			Resolved: false,
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 80)
	out := buf.String()

	// Should not contain reply blockquote markers beyond the body.
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "> ") {
			t.Errorf("unexpected blockquote line in output without replies: %q", line)
		}
	}
}

func TestRenderMarkdownWordWrap(t *testing.T) {
	results := []review.UnresolvedComment{
		{
			Type:     "comment",
			Author:   "alice",
			Body:     "This is a long comment that should be wrapped at word boundaries when the width is narrow enough",
			URL:      "https://github.com/example/1",
			Category: "suggestion",
			Resolved: false,
		},
	}

	var buf bytes.Buffer
	RenderMarkdown(&buf, results, newTestOutput(), 40)
	out := buf.String()

	// Body lines should not exceed the width.
	for _, line := range strings.Split(out, "\n") {
		// Skip header/URL lines (they are not wrapped).
		if strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "##") || strings.Contains(line, "https://") || strings.HasPrefix(line, "---") {
			continue
		}
		if len(line) > 40 {
			t.Errorf("line exceeds width 40: %q (len=%d)", line, len(line))
		}
	}
}
