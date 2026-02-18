# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gh-pr-reviews is a GitHub CLI (`gh`) extension that identifies unresolved review comments in a pull request. It uses the Copilot SDK to classify comments (suggestion, nitpick, issue, question, approval, informational) and determine resolution status. Installed/run as `gh pr-reviews`.

## Common Commands

- **Build:** `go build -o gh-pr-reviews .`
- **Test:** `make test` (runs `go test ./... -coverprofile=coverage.out -covermode=count -count=1`)
- **Single test:** `go test ./path/to/pkg -run TestName`
- **Lint:** `make lint` (runs `golangci-lint run ./...`)
- **CI locally:** `make ci` (installs dev deps + runs tests)

## Architecture

The data flow is: CLI argument → `gh pr view` (PR identification) → GraphQL API (fetch reviews) → Copilot SDK (classify comments) → colored Markdown output to stdout (or JSON with `--json`).

- `main.go` — Entry point, delegates to `cmd.Execute()`
- `cmd/root.go` — Cobra root command. Resolves PR context by shelling out to `gh pr view`, orchestrates the full pipeline with spinner progress. Flags: `-R`, `-a`, `--json`, `--copilot-model`, `--verbose`
- `output/markdown.go` — Colored Markdown-style terminal output using `termenv`. Groups threads by file path, renders PR comments separately. Colors follow GitHub Copilot brand palette and auto-degrade based on terminal capability (`NO_COLOR`, non-TTY)
- `gh/gh.go` — GitHub GraphQL client using `go-github-client` factory for auth and `shurcooL/githubv4` for queries. Fetches `reviewThreads` (inline, with `databaseId` for REST API compatibility) and `comments` (PR-level) with cursor-based pagination
- `review/review.go` — Core data types (`Thread`, `Comment`, `Data`), `CommentClassifier` interface, and `Analyze` function that builds classifier input, calls the classifier, and filters results based on resolution status. Output has two types: `thread` (with `thread_id`) and `comment` (without `thread_id`), both with `comment_id` (REST API numeric ID)
- `review/copilot.go` — `CopilotClassifier` implementation using the Copilot SDK. Sends all PR comments as structured JSON in a single request, receives classification+resolution results. Includes copilot CLI version check (>= 0.0.411 required)
- `version/version.go` — Version constant for tagpr

### Key Design Decisions

- **CommentClassifier interface** in `review/review.go` enables testing with mock classifiers without requiring Copilot
- **GitHub-resolved threads always win**: if `isResolved` is true on GitHub, the thread is treated as resolved regardless of Copilot's classification
- **Single Copilot call**: all comments are sent as one structured JSON payload to minimize API calls
- PR identification delegates to `gh pr view` (supports PR number, URL, or current branch)

## Linting Rules

golangci-lint with these enabled linters: errorlint, godot, gosec, misspell (US locale), revive, funcorder. See `.golangci.yml` for details.

## Prerequisites

- GitHub Copilot CLI >= 0.0.411 must be installed (`copilot --version` to check, `copilot update` to upgrade)

## Release Process

Uses [tagpr](https://github.com/Songmu/tagpr) for automated releases. Version is managed in `version/version.go`. Tags use `v` prefix. Release binaries are built via `gh-extension-precompile`.
