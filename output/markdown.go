package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/k1LoW/gh-pr-reviews/review"
	"github.com/muesli/reflow/wordwrap"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

var (
	colorCopilotPurple = "#8534F3"
	colorPurpleLight   = "#C898FD"
	colorOrange        = "#F08A3A"
	colorOrangeBright  = "#FE4C25"
	colorLink          = "#58A6FF"
)

const maxWidth = 120
const defaultWidth = 80

// DetectWidth returns the appropriate output width.
// If widthFlag > 0, it is used as-is. Otherwise, terminal width is detected
// (capped at 120) with a fallback of 80 for non-TTY.
func DetectWidth(widthFlag int) int {
	if widthFlag > 0 {
		return widthFlag
	}
	fd := int(os.Stdout.Fd()) //nolint:gosec // Fd() returns a small file descriptor.
	if term.IsTerminal(fd) {
		if w, _, err := term.GetSize(fd); err == nil && w > 0 {
			if w > maxWidth {
				return maxWidth
			}
			return w
		}
	}
	return defaultWidth
}

// RenderMarkdown writes review results in a colored Markdown-style format.
func RenderMarkdown(w io.Writer, results []review.UnresolvedComment, p *termenv.Output, width int) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No unresolved comments found.")
		return
	}

	// Separate threads (grouped by path) and PR comments.
	type group struct {
		path     string
		comments []review.UnresolvedComment
	}
	var groups []group
	groupIdx := map[string]int{}
	var prComments []review.UnresolvedComment

	for _, r := range results {
		if r.Type == "thread" {
			idx, ok := groupIdx[r.Path]
			if !ok {
				idx = len(groups)
				groupIdx[r.Path] = idx
				groups = append(groups, group{path: r.Path})
			}
			groups[idx].comments = append(groups[idx].comments, r)
		} else {
			prComments = append(prComments, r)
		}
	}

	first := true

	for _, g := range groups {
		if !first {
			fmt.Fprintln(w)
		}
		first = false

		// File path header.
		header := p.String("## " + g.path).Bold().Foreground(p.Color(colorCopilotPurple))
		fmt.Fprintln(w, header)
		fmt.Fprintln(w)

		for i, c := range g.comments {
			renderComment(w, c, p, width)
			if i < len(g.comments)-1 {
				fmt.Fprintln(w, p.String("---").Faint())
				fmt.Fprintln(w)
			}
		}
	}

	if len(prComments) > 0 {
		if !first {
			fmt.Fprintln(w)
		}
		header := p.String("## PR Comments").Bold().Foreground(p.Color(colorCopilotPurple))
		fmt.Fprintln(w, header)
		fmt.Fprintln(w)

		for i, c := range prComments {
			renderComment(w, c, p, width)
			if i < len(prComments)-1 {
				fmt.Fprintln(w, p.String("---").Faint())
				fmt.Fprintln(w)
			}
		}
	}
}

func renderComment(w io.Writer, c review.UnresolvedComment, p *termenv.Output, width int) {
	// Category label.
	cat := p.String(c.Category).Foreground(p.Color(categoryColor(c.Category)))

	// Status.
	var status termenv.Style
	if c.Resolved {
		status = p.String("(resolved)").Faint()
	} else {
		status = p.String("(unresolved)").Foreground(p.Color(colorOrangeBright))
	}

	// Author.
	author := p.String("@" + c.Author).Bold()

	fmt.Fprintf(w, "### %s %s — %s\n\n", cat, status, author)

	// Location line: line number + URL.
	var parts []string
	if c.Line != nil {
		parts = append(parts, fmt.Sprintf("L%d", *c.Line))
	}
	if c.URL != "" {
		link := p.String(c.URL).Foreground(p.Color(colorLink)).Underline()
		parts = append(parts, link.String())
	}
	if len(parts) > 0 {
		fmt.Fprintln(w, strings.Join(parts, " | "))
	}

	// Body.
	fmt.Fprintln(w, wordwrap.String(c.Body, width))

}

func categoryColor(category string) string {
	switch category {
	case "question", "nitpick":
		return colorOrange
	default:
		return colorPurpleLight
	}
}
