package review

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// copilotReviewerLogin is the GitHub login of the Copilot code reviewer.
// GraphQL reports it bare while REST appends a "[bot]" suffix.
const copilotReviewerLogin = "copilot-pull-request-reviewer"

// SubmittedReview represents a submitted review together with its summary body.
type SubmittedReview struct {
	ID          string    `json:"id"`
	Body        string    `json:"body"`
	Author      string    `json:"author"`
	SubmittedAt time.Time `json:"submitted_at"`
	URL         string    `json:"url"`
}

// SuppressedComment represents one entry of the "Suppressed comments" section in
// a Copilot review body. GitHub holds no review thread for these, so they carry
// no resolution state and cannot be replied to or resolved.
type SuppressedComment struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	Line       *int      `json:"line,omitempty"`
	Body       string    `json:"body"`
	Snippet    string    `json:"snippet,omitempty"`
	Author     string    `json:"author"`
	CreatedAt  time.Time `json:"created_at"`
	URL        string    `json:"url"`
	IsOutdated bool      `json:"is_outdated"`
}

// ExtractSuppressedComments splits the "Suppressed comments" sections of Copilot
// review bodies into individual comments.
//
// Copilot re-emits the full list of still-relevant findings on every re-review,
// so only the newest review describes the current code. Entries from earlier
// reviews are kept but flagged outdated rather than dropped, so that -a can
// still surface them.
func ExtractSuppressedComments(reviews []SubmittedReview) []SuppressedComment {
	type parsedReview struct {
		review  SubmittedReview
		entries []suppressedEntry
	}

	var parsed []parsedReview
	newest := -1
	for _, r := range reviews {
		if !isCopilotReviewer(r.Author) {
			continue
		}
		entries := parseSuppressedSection(r.Body)
		if len(entries) == 0 {
			continue
		}
		if newest < 0 || r.SubmittedAt.After(parsed[newest].review.SubmittedAt) {
			newest = len(parsed)
		}
		parsed = append(parsed, parsedReview{review: r, entries: entries})
	}

	var out []SuppressedComment
	for i, p := range parsed {
		for j, e := range p.entries {
			out = append(out, SuppressedComment{
				ID:         fmt.Sprintf("%s#suppressed-%d", p.review.ID, j),
				Path:       e.path,
				Line:       e.line,
				Body:       e.body,
				Snippet:    e.snippet,
				Author:     p.review.Author,
				CreatedAt:  p.review.SubmittedAt,
				URL:        p.review.URL,
				IsOutdated: i != newest,
			})
		}
	}
	return out
}

func isCopilotReviewer(login string) bool {
	return strings.TrimSuffix(login, "[bot]") == copilotReviewerLogin
}

type suppressedEntry struct {
	path    string
	line    *int
	body    string
	snippet string
}

var (
	suppressedHeadingRe = regexp.MustCompile(`^#{1,6}\s+Suppressed comments\b`)
	suppressedEntryRe   = regexp.MustCompile(`^\*\*(.+):(\d+)\*\*$`)
	filesReviewedRe     = regexp.MustCompile(`^[-*]\s+\*\*Files reviewed:`)
)

// parseSuppressedSection scans line by line instead of matching the whole
// section at once because entry bodies embed fenced code that can otherwise
// look like a heading or an end-of-section marker.
func parseSuppressedSection(body string) []suppressedEntry {
	var (
		entries []suppressedEntry
		cur     *suppressedEntry
		buf     []string
		inFence bool
		started bool
	)

	flush := func() {
		if cur == nil {
			return
		}
		cur.body, cur.snippet = splitEntryBody(buf)
		if cur.body != "" || cur.snippet != "" {
			entries = append(entries, *cur)
		}
		cur, buf = nil, nil
	}

	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			if started && cur != nil {
				buf = append(buf, line)
			}
			continue
		}
		if !started {
			if !inFence && suppressedHeadingRe.MatchString(line) {
				started = true
			}
			continue
		}
		if !inFence {
			if strings.HasPrefix(line, "#") ||
				strings.HasPrefix(strings.TrimSpace(line), "</details>") ||
				filesReviewedRe.MatchString(line) {
				break
			}
			if m := suppressedEntryRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				flush()
				cur = &suppressedEntry{path: strings.TrimSpace(m[1])}
				if n, err := strconv.Atoi(m[2]); err == nil {
					cur.line = &n
				}
				continue
			}
		}
		if cur != nil {
			buf = append(buf, line)
		}
	}
	flush()

	return entries
}

// splitEntryBody separates the prose of a suppressed comment from the trailing
// fenced code snippet Copilot attaches to it.
func splitEntryBody(lines []string) (body, snippet string) {
	fence := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fence = i
			break
		}
	}

	text := lines
	if fence >= 0 {
		text = lines[:fence]
		var snip []string
		for _, l := range lines[fence+1:] {
			if strings.HasPrefix(strings.TrimSpace(l), "```") {
				break
			}
			snip = append(snip, l)
		}
		snippet = strings.TrimRight(strings.Join(snip, "\n"), "\n")
	}

	return strings.TrimSpace(stripBullet(text)), snippet
}

// stripBullet removes the list marker Copilot puts on the first line of an entry.
func stripBullet(lines []string) string {
	joined := strings.TrimLeft(strings.Join(lines, "\n"), "\n")
	for _, marker := range []string{"* ", "- ", "+ "} {
		if after, ok := strings.CutPrefix(joined, marker); ok {
			return after
		}
	}
	return joined
}
