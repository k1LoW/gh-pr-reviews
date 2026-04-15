package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	copilot "github.com/github/copilot-sdk/go"
)

const minCopilotVersion = "0.0.411"

const systemPrompt = `You are a code review comment classifier. You analyze PR review comments and classify each one.

IMPORTANT: Each thread contains a "comments" array listing the full conversation in chronological order. The first comment is the original review comment; subsequent comments are replies. You MUST read ALL comments in every thread before classifying. The conversation flow is critical for determining resolution status.

For each comment or thread, determine:
1. **category**: You MUST always choose exactly one of: "suggestion", "nitpick", "issue", "question", "approval", "informational". Never return any other value.
   - suggestion: Code change proposals or improvement requests ("you should fix this", "this pattern would be better")
   - nitpick: Minor style/formatting/naming issues (not blockers but suggest changes)
   - issue: Bug reports or problem identification ("this will break when...")
   - question: Questions about the code ("why did you do this?")
   - approval: Approval comments ("LGTM", "looks good")
   - informational: FYI, context, or background information. If a comment does not clearly fit into "suggestion", "nitpick", "issue", or "question", classify it as "informational"

2. **is_resolved**: Whether the feedback has been addressed. Only evaluate resolution for "suggestion", "nitpick", "issue", and "question" categories. For "approval" and "informational", always set is_resolved to true.
   - For threads with multiple comments, you MUST analyze the ENTIRE conversation to determine resolution:
     - For "suggestion", "nitpick", and "issue": A reply from the PR author acknowledging the feedback (e.g., "fixed", "done", "updated", explaining the change, or providing a valid justification for the current approach) indicates resolution.
     - For "question": Any substantive reply that answers the question indicates resolution. This includes explanations, clarifications, or references to relevant context. A question that has received a meaningful answer MUST be marked as resolved.
   - If is_resolved_on_github is true, always consider it resolved regardless of comment content.
   - Only set is_resolved to false when there is NO meaningful reply addressing the concern, or when there is an ongoing unresolved disagreement after the latest reply.

3. **reason**: Brief explanation of your classification and resolution decision. When a thread has multiple comments, reference the relevant reply that influenced your decision.

You will receive a JSON object with "threads" (inline review threads) and "pr_comments" (top-level PR comments).

Return a JSON object (no markdown fences) with the same structure, adding category, is_resolved, and reason fields:
{
  "threads": [{"thread_id": "...", "category": "...", "is_resolved": true/false, "reason": "..."}],
  "pr_comments": [{"id": "...", "category": "...", "is_resolved": true/false, "reason": "..."}]
}

Return ONLY valid JSON. Do not wrap in markdown code fences.`

// CopilotClassifier uses the Copilot SDK to classify review comments.
type CopilotClassifier struct {
	client  *copilot.Client
	session *copilot.Session
}

// NewCopilotClassifier creates a new CopilotClassifier.
func NewCopilotClassifier(ctx context.Context, model string) (*CopilotClassifier, error) {
	if err := checkCopilotCLI(); err != nil {
		return nil, err
	}

	client := copilot.NewClient(&copilot.ClientOptions{
		LogLevel: "error",
	})

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start copilot client: %w", err)
	}

	session, err := client.CreateSession(ctx, &copilot.SessionConfig{
		OnPermissionRequest: copilot.PermissionHandler.ApproveAll,
		Model:               model,
		SystemMessage: &copilot.SystemMessageConfig{
			Content: systemPrompt,
		},
	})
	if err != nil {
		client.Stop() //nolint:errcheck
		return nil, fmt.Errorf("failed to create copilot session: %w", err)
	}

	return &CopilotClassifier{
		client:  client,
		session: session,
	}, nil
}

// ClassifyAll sends all review data to Copilot and returns classification results.
func (c *CopilotClassifier) ClassifyAll(ctx context.Context, input *ClassifyInput) (*ClassifyOutput, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal classify input: %w", err)
	}

	var responseContent string
	done := make(chan struct{})
	var eventErr error

	unsubscribe := c.session.On(func(event copilot.SessionEvent) {
		switch event.Type {
		case "assistant.message":
			if d, ok := event.Data.(*copilot.AssistantMessageData); ok {
				responseContent = d.Content
			}
		case "session.idle":
			close(done)
		case "error":
			if d, ok := event.Data.(*copilot.SessionErrorData); ok {
				eventErr = fmt.Errorf("copilot error: %s", d.Message)
			}
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})
	defer unsubscribe()

	_, err = c.session.Send(ctx, copilot.MessageOptions{
		Prompt: string(inputJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send message to copilot: %w", err)
	}

	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if eventErr != nil {
		return nil, eventErr
	}

	output, err := parseClassifyOutput(responseContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse copilot response: %w", err)
	}

	return output, nil
}

// Close shuts down the Copilot session and client.
func (c *CopilotClassifier) Close() {
	if c.session != nil {
		c.session.Disconnect() //nolint:errcheck
	}
	if c.client != nil {
		c.client.Stop() //nolint:errcheck
	}
}

func checkCopilotCLI() error {
	out, err := exec.Command("copilot", "--version").Output()
	if err != nil {
		return fmt.Errorf("copilot CLI not found. Please install GitHub Copilot CLI >= %s", minCopilotVersion)
	}

	version := parseCopilotVersion(string(out))
	if version == "" {
		return fmt.Errorf("could not parse copilot CLI version from: %s", strings.TrimSpace(string(out)))
	}

	if compareVersions(version, minCopilotVersion) < 0 {
		return fmt.Errorf("copilot CLI version %s is too old. Please update to >= %s (run: copilot update)", version, minCopilotVersion)
	}

	return nil
}

var versionRegexp = regexp.MustCompile(`(\d+\.\d+\.\d+)`)

func parseCopilotVersion(output string) string {
	m := versionRegexp.FindString(output)
	return m
}

// compareVersions compares two semver-like version strings (e.g., "0.0.411" vs "0.0.410").
// Returns -1, 0, or 1.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := range max(len(aParts), len(bParts)) {
		var av, bv int
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func parseClassifyOutput(raw string) (*ClassifyOutput, error) {
	raw = strings.TrimSpace(raw)
	// Defensively strip markdown code fences.
	if strings.HasPrefix(raw, "```") {
		lines := strings.SplitN(raw, "\n", 2)
		if len(lines) > 1 {
			raw = lines[1]
		}
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	var output ClassifyOutput
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal classify output: %w (raw: %s)", err, truncate(raw, 200))
	}
	return &output, nil
}

// ListCopilotModels returns available model IDs from the Copilot SDK.
func ListCopilotModels(ctx context.Context) ([]string, error) {
	if err := checkCopilotCLI(); err != nil {
		return nil, err
	}

	client := copilot.NewClient(&copilot.ClientOptions{
		LogLevel: "error",
	})

	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start copilot client: %w", err)
	}
	defer client.Stop() //nolint:errcheck

	models, err := client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
