---
name: triage-pr-reviews
description: >
  Triages unresolved PR review comments using gh-pr-reviews.
  Analyzes code context and classifies each comment as Agree / Partially Agree / Disagree.
  Walks through each comment one-by-one, asking the user what action to take.
  Use when the user wants to triage, review, or analyze unresolved PR comments.
compatibility: Requires gh CLI and gh-pr-reviews extension (gh extension install k1LoW/gh-pr-reviews)
---

# Triage PR Review Comments

## Phase 1: Fetch and Analyze

1. Run `gh pr-reviews [arg] --json` to get unresolved review comments as JSON. If no argument is given, use the current branch's PR. Note: this command uses Copilot for classification and may take a while depending on the number of comments — use a longer timeout. Each JSON object contains:
   - `comment_id` (int): REST API comment ID — usable for replying via `gh api`
   - `thread_id` (string, only for `type: "thread"`): inline review thread ID
   - `type`: `"thread"` (inline review) or `"comment"` (PR-level)
   - `author`, `body`, `url`: comment metadata
   - `commit_id`, `path`, `line`, `diff_hunk` (only for `type: "thread"`): file location and diff context
   - `category`: one of `suggestion`, `nitpick`, `issue`, `question`, `approval`, `informational`
   - `resolved` (bool), `reason` (string): resolution status and rationale
2. Check if PR metadata (number, title, url) is already available from conversation context. If not (e.g., when a PR number/URL is explicitly passed as argument), run `gh pr view [arg] --json number,title,url` to get it.
3. For `type: "thread"` comments, use `path`, `line`, and `diff_hunk` from the JSON response to identify the exact file location. For `type: "comment"` (PR-level), there is no file location.
4. Check code context for each comment. Leverage any existing conversation context first. Only fetch additional context via `gh pr diff` or file reads when necessary.
5. Evaluate each comment against the code context. Classify as **Agree**, **Partially Agree**, or **Disagree** with a rationale and suggested action.

## Phase 2: Summary Overview

Show a brief summary of all comments before starting the interactive walkthrough:

```
## Unresolved Review Comments — PR #<number> (<title>)

| # | Category | Author | Assessment | File |
|---|----------|--------|------------|------|
| 1 | <category> | @<author> | Agree/Partially Agree/Disagree | `<path>:<line>` |
| 2 | <category> | @<author> | Agree/Partially Agree/Disagree | `PR-level` |

Total: <count> comments — Agree: n, Partially Agree: n, Disagree: n

Walking through each comment below...
```

## Phase 3: Interactive Walkthrough (one-by-one)

For each comment, in order:

1. **Present the comment** in this format:

```
---
### [<current>/<total>] [<category>] by @<author>
> <comment body>

**File**: `<path>` (line <line>)   ← omit for PR-level comments
**Assessment**: <Agree | Partially Agree | Disagree>
**Rationale**: <1-3 sentences>
**Suggested action**: <recommended action>
```

2. **Ask the user what to do**. Present the following action choices and wait for the user's response before proceeding. The user may pick one of the predefined actions or provide free-text instructions:

   - For `type: "thread"` (inline review thread), offer:
     1. **Fix in code** — Make the code change only
     2. **Fix & reply & resolve** — Make the code change, post a reply, and resolve the thread
     3. **Fix & reply** — Make the code change and post a reply without resolving
     4. **Reply & resolve** — Post a reply comment on GitHub and resolve the thread
     5. **Reply only** — Post a reply comment on GitHub without resolving
     6. **Skip** — Move on without taking action
   - For `type: "comment"` (PR-level comment), offer:
     1. **Fix in code** — Make the code change only
     2. **Fix & reply** — Make the code change and post a reply
     3. **Reply only** — Post a reply comment on GitHub
     4. **Skip** — Move on without taking action
   - The user may select by number, name, or provide **custom instructions** (e.g., "fix but also refactor the surrounding function", "reply with a question asking for clarification", etc.)

3. **Execute the chosen action**:
   - **Fix in code**: Make the code change only.
   - **Fix & reply & resolve** (`type: "thread"` only): Make the code change, ask the user what to reply (or suggest a draft reply), post the reply via `gh api`, and resolve the thread via `gh api`.
   - **Fix & reply**: Make the code change, ask the user what to reply (or suggest a draft reply), and post the reply via `gh api`.
   - **Reply & resolve** (`type: "thread"` only): Ask the user what to reply (or suggest a draft reply). Then post the reply via `gh api` and resolve the thread via `gh api`.
   - **Reply only**: Ask the user what to reply (or suggest a draft reply). Then post the reply via `gh api`.
   - **Skip**: Do nothing, proceed to the next comment.
   - **Other (free-text)**: Follow the user's custom instructions for this comment. This may combine multiple actions or request something not covered by the predefined options.

4. After completing the action (or skipping), move to the next comment and repeat.

## Phase 4: Final Summary

After all comments have been walked through, show a final summary:

```
## Triage Complete

| # | Category | Author | Assessment | Action Taken |
|---|----------|--------|------------|--------------|
| 1 | <category> | @<author> | <assessment> | Fixed / Fixed & replied & resolved / Fixed & replied / Replied & resolved / Replied / Skipped |
| 2 | ... | ... | ... | ... |

- Fixed: n
- Fixed & replied & resolved: n
- Fixed & replied: n
- Replied & resolved: n
- Replied only: n
- Skipped: n
```

## GitHub API Reference

- **Reply to an inline review comment (thread)**:
  `gh api repos/{owner}/{repo}/pulls/{pull_number}/comments/{comment_id}/replies -f body="<reply>"`
- **Post a PR-level comment (issue comment)**:
  `gh api repos/{owner}/{repo}/issues/{pull_number}/comments -f body="<reply>"`
- **Resolve a review thread**:
  `gh api graphql -f query='mutation { resolveReviewThread(input: {threadId: "<thread_id>"}) { thread { id } } }'`

## Rules

- Do NOT commit or push unless the user explicitly requests it.
- When fixing code, make minimal changes that address the review comment.
- When suggesting reply drafts, keep them concise and professional.
- If code context is unclear, search the codebase to verify before making a judgment.
- Prefer `gh` commands for GitHub data.
