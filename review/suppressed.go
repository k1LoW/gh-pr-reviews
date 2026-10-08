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

// SuppressedComment represents one finding Copilot listed in a review body
// instead of posting inline, taken from either the "Suppressed comments" or the
// "Previously missed" section. GitHub holds no review thread for these, so they
// carry no resolution state and cannot be replied to or resolved.
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
	// OutdatedReason explains why IsOutdated is set.
	OutdatedReason string `json:"-"`
}

// ExtractSuppressedComments splits the "Suppressed comments" and "Previously
// missed" sections of Copilot review bodies into individual comments.
//
// Copilot re-emits the full list of still-relevant suppressed findings on every
// re-review, so only the newest review describes the current code. Suppressed
// entries from earlier reviews are kept but flagged outdated rather than
// dropped, so that -a can still surface them.
//
// "Previously missed" entries are per-review discoveries in code left unchanged
// since the prior review, and Copilot does not re-list them each time, so the
// newest-review rule would hide findings that were never addressed. Only exact
// path:line duplicates across reviews are flagged outdated for them.
func ExtractSuppressedComments(reviews []SubmittedReview) []SuppressedComment {
	type parsedReview struct {
		review  SubmittedReview
		entries []suppressedEntry
		missed  []suppressedEntry
	}

	var parsed []parsedReview
	newest := -1
	for _, r := range reviews {
		if !isCopilotReviewer(r.Author) {
			continue
		}
		entries := parseSuppressedSection(r.Body)
		missed := parsePreviouslyMissedSection(r.Body)
		if len(entries) == 0 && len(missed) == 0 {
			continue
		}
		parsed = append(parsed, parsedReview{review: r, entries: entries, missed: missed})
		if len(entries) > 0 && (newest < 0 || r.SubmittedAt.After(parsed[newest].review.SubmittedAt)) {
			newest = len(parsed) - 1
		}
	}

	latestMissed := map[missedKey]time.Time{}
	for _, p := range parsed {
		for _, e := range p.missed {
			k := missedLocation(e)
			if t, ok := latestMissed[k]; !ok || p.review.SubmittedAt.After(t) {
				latestMissed[k] = p.review.SubmittedAt
			}
		}
	}

	var out []SuppressedComment
	for i, p := range parsed {
		for j, e := range p.entries {
			c := newSuppressedComment(p.review, e, fmt.Sprintf("%s#suppressed-%d", p.review.ID, j))
			if i != newest {
				c.IsOutdated = true
				c.OutdatedReason = supersededReason
			}
			out = append(out, c)
		}
		for j, e := range p.missed {
			c := newSuppressedComment(p.review, e, fmt.Sprintf("%s#previously-missed-%d", p.review.ID, j))
			// Within one review the same location can hold distinct findings,
			// so only a later review counts as re-reporting it.
			if p.review.SubmittedAt.Before(latestMissed[missedLocation(e)]) {
				c.IsOutdated = true
				c.OutdatedReason = reportedAgainReason
			}
			out = append(out, c)
		}
	}
	return out
}

func newSuppressedComment(r SubmittedReview, e suppressedEntry, id string) SuppressedComment {
	return SuppressedComment{
		ID:        id,
		Path:      e.path,
		Line:      e.line,
		Body:      e.body,
		Snippet:   e.snippet,
		Author:    r.Author,
		CreatedAt: r.SubmittedAt,
		URL:       r.URL,
	}
}

type missedKey struct {
	path string
	line int
}

func missedLocation(e suppressedEntry) missedKey {
	k := missedKey{path: e.path}
	if e.line != nil {
		k.line = *e.line
	}
	return k
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

var (
	previouslyMissedSummaryRe = regexp.MustCompile(`(?i)<summary>\s*<strong>\s*(?:previously missed\b|\d+\s+previously missed\b)`)
	missedLocationRe          = regexp.MustCompile("^`([^`]+):(\\d+)`$")
	htmlTagRe                 = regexp.MustCompile(`<[^>]*>`)
)

// parsePreviouslyMissedSection reads the "Previously missed" block of the v2
// Copilot review overview. Each finding is its own nested <details>, so the
// section end is found by tracking nesting depth rather than stopping at the
// first </details>.
func parsePreviouslyMissedSection(body string) []suppressedEntry {
	var (
		entries []suppressedEntry
		cur     *suppressedEntry
		title   string
		buf     []string
		inFence bool
		started bool
		depth   int
	)

	flush := func() {
		if cur == nil {
			return
		}
		prose := strings.TrimSpace(strings.Join(buf, "\n"))
		switch {
		case title != "" && prose != "":
			cur.body = title + "\n\n" + prose
		case title != "":
			cur.body = title
		default:
			cur.body = prose
		}
		if cur.path != "" && cur.body != "" {
			entries = append(entries, *cur)
		}
		cur, title, buf = nil, "", nil
	}

	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			if cur != nil {
				buf = append(buf, line)
			}
			continue
		}
		if inFence {
			if cur != nil {
				buf = append(buf, line)
			}
			continue
		}
		if !started {
			if previouslyMissedSummaryRe.MatchString(line) {
				started = true
				depth = 1
			}
			continue
		}

		if strings.HasPrefix(trimmed, "<details") {
			depth++
			if depth == 2 {
				flush()
				cur = &suppressedEntry{}
			}
			trimmed = strings.TrimSpace(trimmed[strings.Index(trimmed, ">")+1:])
			if trimmed == "" {
				continue
			}
		}
		if strings.HasPrefix(trimmed, "</details>") {
			if depth == 2 {
				flush()
			}
			depth--
			if depth <= 0 {
				break
			}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(trimmed, "<summary>") && title == "" {
			title = strings.TrimSpace(htmlTagRe.ReplaceAllString(trimmed, ""))
			continue
		}
		if cur.path == "" {
			if m := missedLocationRe.FindStringSubmatch(trimmed); m != nil {
				cur.path = strings.TrimSpace(strings.ReplaceAll(m[1], "\u200b", ""))
				if n, err := strconv.Atoi(m[2]); err == nil {
					cur.line = &n
				}
				continue
			}
		}
		buf = append(buf, line)
	}
	flush()

	return entries
}
