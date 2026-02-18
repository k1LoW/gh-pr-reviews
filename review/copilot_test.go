package review

import (
	"testing"
)

func TestParseClassifyOutput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		check   func(t *testing.T, o *ClassifyOutput)
	}{
		{
			name: "plain JSON",
			raw:  `{"threads":[{"thread_id":"T1","category":"suggestion","is_resolved":false,"reason":"not fixed"}],"pr_comments":[]}`,
			check: func(t *testing.T, o *ClassifyOutput) {
				t.Helper()
				if len(o.Threads) != 1 {
					t.Fatalf("expected 1 thread, got %d", len(o.Threads))
				}
				if o.Threads[0].Category != "suggestion" {
					t.Errorf("expected category suggestion, got %s", o.Threads[0].Category)
				}
				if o.Threads[0].IsResolved {
					t.Error("expected IsResolved to be false")
				}
			},
		},
		{
			name: "markdown fenced JSON",
			raw: "```json\n{\"threads\":[],\"pr_comments\":[{\"id\":\"PC1\",\"category\":\"question\",\"is_resolved\":true,\"reason\":\"answered\"}]}\n```",
			check: func(t *testing.T, o *ClassifyOutput) {
				t.Helper()
				if len(o.PRComments) != 1 {
					t.Fatalf("expected 1 PR comment, got %d", len(o.PRComments))
				}
				if o.PRComments[0].Category != "question" {
					t.Errorf("expected category question, got %s", o.PRComments[0].Category)
				}
			},
		},
		{
			name: "markdown fenced without language",
			raw:  "```\n{\"threads\":[],\"pr_comments\":[]}\n```",
			check: func(t *testing.T, o *ClassifyOutput) {
				t.Helper()
				if len(o.Threads) != 0 {
					t.Errorf("expected 0 threads, got %d", len(o.Threads))
				}
			},
		},
		{
			name:    "invalid JSON",
			raw:     "not json at all",
			wantErr: true,
		},
		{
			name:    "empty string",
			raw:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := parseClassifyOutput(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, output)
			}
		})
	}
}

func TestParseCopilotVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"GitHub Copilot CLI 0.0.410.\n", "0.0.410"},
		{"GitHub Copilot CLI 0.0.411.\nRun 'copilot update' to check for updates.\n", "0.0.411"},
		{"1.2.3", "1.2.3"},
		{"no version here", ""},
	}
	for _, tt := range tests {
		got := parseCopilotVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseCopilotVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.0.411", "0.0.411", 0},
		{"0.0.412", "0.0.411", 1},
		{"0.0.410", "0.0.411", -1},
		{"0.1.0", "0.0.999", 1},
		{"1.0.0", "0.99.99", 1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("expected hello, got %s", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("expected hello..., got %s", got)
	}
}
