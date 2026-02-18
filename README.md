# gh-pr-reviews

`gh-pr-reviews` is a GitHub CLI (`gh`) extension that identifies unresolved review comments in a pull request.

It uses the [Copilot SDK](https://github.com/github/copilot-sdk) to classify each comment (suggestion, nitpick, issue, question, approval, informational) and determine whether it has been resolved.

## Usage

```bash
# Current branch's PR
$ gh pr-reviews

# Specific PR number
$ gh pr-reviews 123

# Specific repository
$ gh pr-reviews --repo owner/repo 123

# Show all comments including resolved ones
$ gh pr-reviews --all
```

### Output

By default, results are displayed in a colored Markdown-style format grouped by file path. Colors follow the GitHub Copilot brand palette and are automatically disabled when output is piped or `NO_COLOR` is set.

```
## src/handler.go

### suggestion (unresolved) — @reviewer

L42 | https://github.com/owner/repo/pull/123#discussion_r123456

This should use error wrapping

## PR Comments

### suggestion (unresolved) — @reviewer

https://github.com/owner/repo/pull/123#issuecomment-123456

Overall looks good but please address the error handling
```

Use `--json` to get machine-readable JSON output:

```bash
$ gh pr-reviews 123 --json
```

There are two types: `thread` (inline review thread) and `comment` (PR-level comment). `thread_id`, `path`, `line`, `commit_id`, and `diff_hunk` are only present for `thread` type. `comment_id` is the REST API comment ID, which can be used for replying.

```json
[
  {
    "thread_id": "PRRT_kwDOH7hXo85vAD-t",
    "comment_id": 2815812186,
    "type": "thread",
    "path": "src/handler.go",
    "line": 42,
    "commit_id": "abc1234def5678",
    "diff_hunk": "@@ -40,6 +40,7 @@ func handleRequest(w http.ResponseWriter, r *http.Request) {\n \tif err != nil {\n-\t\tlog.Println(err)\n+\t\treturn err",
    "author": "reviewer",
    "body": "This should use error wrapping",
    "url": "https://github.com/owner/repo/pull/123#discussion_r123456",
    "category": "suggestion",
    "resolved": false,
    "reason": "No follow-up addressing this feedback"
  },
  {
    "comment_id": 2815800000,
    "type": "comment",
    "author": "reviewer",
    "body": "Overall looks good but please address the error handling",
    "url": "https://github.com/owner/repo/pull/123#issuecomment-123456",
    "category": "suggestion",
    "resolved": false,
    "reason": "No follow-up addressing this feedback"
  }
]
```

### Comment Categories

| Category | Description |
|----------|-------------|
| `suggestion` | Code change proposals or improvement requests |
| `nitpick` | Minor style/formatting/naming issues |
| `issue` | Bug reports or problem identification |
| `question` | Questions about the code |
| `approval` | Approval comments (LGTM, looks good) |
| `informational` | FYI, context, or background information |

Only `suggestion`, `nitpick`, and `issue` categories are evaluated for resolution status. The rest are always treated as resolved.

## Install

```bash
$ gh extension install k1LoW/gh-pr-reviews
```

## Prerequisites

- [GitHub Copilot CLI](https://docs.github.com/en/copilot) >= 0.0.411 (`copilot --version` to check, `copilot update` to upgrade)

## Command Line Options

| Option | Short | Description |
|--------|-------|-------------|
| `--repo` | `-R` | Select another repository using the `[HOST/]OWNER/REPO` format |
| `--all` | `-a` | Show all review comments including resolved ones |
| `--json` | | Output results as JSON |
| `--width` | `-w` | Output width (0 for auto-detect, default: auto) |
| `--copilot-model` | | Copilot model to use for classification (default: `claude-haiku-4.5`) |
| `--verbose` | | Verbose output |

## Contributing

To use this project from source, instead of a release:

    go build .
    gh extension remove pr-reviews
    gh extension install .
