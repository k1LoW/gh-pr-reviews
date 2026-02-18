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

JSON array of review comments with classification and resolution status:

```json
[
  {
    "thread_id": "PRRT_kwDOH7hXo85vAD-t",
    "type": "thread",
    "author": "reviewer",
    "body": "This should use error wrapping",
    "url": "https://github.com/owner/repo/pull/123#discussion_r123456",
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
| `--copilot-model` | | Copilot model to use for classification (default: `gpt-4o`) |
| `--verbose` | | Verbose output |

## Contributing

To use this project from source, instead of a release:

    go build .
    gh extension remove pr-reviews
    gh extension install .
