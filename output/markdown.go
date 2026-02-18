package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/k1LoW/gh-pr-reviews/review"
	"github.com/muesli/termenv"
)

var (
	colorCopilotPurple = "#8534F3"
	colorPurpleLight   = "#C898FD"
	colorOrange        = "#F08A3A"
	colorOrangeBright  = "#FE4C25"
	colorPurpleDark    = "#43179E"
)

// RenderMarkdown writes review results in a colored Markdown-style format.
func RenderMarkdown(w io.Writer, results []review.UnresolvedComment, p *termenv.Output) {
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
			renderComment(w, c, p)
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
			renderComment(w, c, p)
			if i < len(prComments)-1 {
				fmt.Fprintln(w, p.String("---").Faint())
				fmt.Fprintln(w)
			}
		}
	}
}

func renderComment(w io.Writer, c review.UnresolvedComment, p *termenv.Output) {
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

	fmt.Fprintf(w, "### %s %s — %s\n", cat, status, author)

	// Location line: line number + URL.
	var parts []string
	if c.Line != nil {
		parts = append(parts, fmt.Sprintf("L%d", *c.Line))
	}
	if c.URL != "" {
		link := p.String(c.URL).Foreground(p.Color(colorPurpleDark)).Underline()
		parts = append(parts, link.String())
	}
	if len(parts) > 0 {
		fmt.Fprintln(w, strings.Join(parts, " | "))
	}

	// Body.
	fmt.Fprintln(w, c.Body)
	fmt.Fprintln(w)
}

func categoryColor(category string) string {
	switch category {
	case "question", "nitpick":
		return colorOrange
	default:
		return colorPurpleLight
	}
}
