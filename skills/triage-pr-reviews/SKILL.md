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
     2. **Fix & comment** — Make the code change and post a PR-level comment
     3. **Comment only** — Post a PR-level comment on GitHub
     4. **Skip** — Move on without taking action
   - The user may select by number, name, or provide **custom instructions** (e.g., "fix but also refactor the surrounding function", "reply with a question asking for clarification", etc.)

3. **Execute the chosen action** — code fixes and commits are applied immediately during the walkthrough. **GitHub API actions (reply, comment, resolve) are deferred** to Phase 4 (ideally after pushing). During this phase, confirm the reply/comment content with the user and queue it for later execution. When an action includes a code fix, record the commit hash so it can be referenced in the queued reply/comment.
   - **Fix in code**: Make the code change, draft a commit message following the repository's commit message conventions (check `git log` for style), confirm it with the user, and commit.
   - **Fix & reply & resolve** (`type: "thread"` only): Make the code change, draft a commit message following the repository's commit message conventions (check `git log` for style), confirm it with the user, and commit. Ask the user what to reply (or suggest a draft reply) and confirm the content. Queue the reply and resolve for Phase 4.
   - **Fix & reply** (`type: "thread"`): Make the code change, draft a commit message following the repository's commit message conventions (check `git log` for style), confirm it with the user, and commit. Ask the user what to reply (or suggest a draft reply) and confirm the content. Queue the reply for Phase 4.
   - **Fix & comment** (`type: "comment"`): Make the code change, draft a commit message following the repository's commit message conventions (check `git log` for style), confirm it with the user, and commit. Ask the user what to comment (or suggest a draft) and confirm the content. Queue the comment for Phase 4.
   - **Reply & resolve** (`type: "thread"` only): Ask the user what to reply (or suggest a draft reply) and confirm the content. Queue the reply and resolve for Phase 4.
   - **Reply only** (`type: "thread"`): Ask the user what to reply (or suggest a draft reply) and confirm the content. Queue the reply for Phase 4.
   - **Comment only** (`type: "comment"`): Ask the user what to comment (or suggest a draft) and confirm the content. Queue the comment for Phase 4.
   - **Skip**: Do nothing, proceed to the next comment.
   - **Other (free-text)**: Follow the user's custom instructions for this comment. Code fixes are committed immediately; any GitHub API actions are queued for Phase 4.

4. After completing the action (or skipping), move to the next comment and repeat.

## Phase 4: Push & Execute Queued Actions

After all comments have been walked through:

1. **Show the triage summary**:

```
## Triage Summary

| # | Category | Author | Assessment | Action |
|---|----------|--------|------------|--------|
| 1 | <category> | @<author> | <assessment> | Fixed (committed) & reply & resolve (queued) / Fixed (committed) / Skipped / ... |
| 2 | ... | ... | ... | ... |

Pending GitHub actions: <n> replies, <n> comments, <n> resolves
```

2. **Ask: "Push?"** — If the user agrees, push to the remote. If the user declines, skip this step.

3. **Execute queued replies/comments/resolves** — Show the list of pending GitHub API actions. For actions that include a code fix, include the commit hash in the reply/comment (e.g., "Fixed in abc1234.").
   - **If push succeeded**: Ask "Execute queued replies/comments/resolves?" and execute if the user agrees.
   - **If push was declined**: Warn that executing replies/resolves before pushing means reviewers cannot see the fixes yet. Require explicit confirmation: "Execute queued replies/comments/resolves without pushing?" Only execute if the user confirms after this warning.
   - Endpoints:
     - For thread replies: `gh api` to `POST /repos/{owner}/{repo}/pulls/{pull_number}/comments/{comment_id}/replies`
     - For PR-level comments: `gh api` to `POST /repos/{owner}/{repo}/issues/{pull_number}/comments`
     - For thread resolves: `gh api graphql` using `thread_id` (not `comment_id`)
   - If the user declines, show the pending actions list so they can execute manually later.

4. **Show the final result**:

```
## Triage Complete

| # | Category | Author | Assessment | Action Taken |
|---|----------|--------|------------|--------------|
| 1 | <category> | @<author> | <assessment> | Fixed & replied & resolved / Fixed / Skipped / ... |
| 2 | ... | ... | ... | ... |

- Fixed: n
- Replied/Commented: n
- Resolved: n
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

- When the user selects a "Fix" action, this is an implicit request to commit. Draft a commit message, confirm it with the user, and commit. Do NOT push unless explicitly confirmed in Phase 4.
- When fixing code, make minimal changes that address the review comment.
- When suggesting reply drafts, keep them concise and professional.
- If code context is unclear, search the codebase to verify before making a judgment.
- Prefer `gh` commands for GitHub data.
