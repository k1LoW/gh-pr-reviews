# Changelog

## [v0.8.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.7.0...v0.8.0) - 2026-04-05
### New Features 🎉
- feat: make triage-pr-reviews skill interactive with one-by-one walkthrough by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/33
### Dependency Updates ⬆️
- chore(deps): bump reviewdog/action-golangci-lint from 2.8.0 to 2.10.0 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/29
- chore(deps): bump golang.org/x/term from 0.40.0 to 0.41.0 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/28
- chore(deps): bump github.com/github/copilot-sdk/go from 0.1.32 to 0.2.0 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/31
- chore(deps): bump actions/setup-go from 6.3.0 to 6.4.0 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/32

## [v0.7.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.6.0...v0.7.0) - 2026-03-13
### Breaking Changes 🛠
- refactor: replace validCategories map with switch and extract default constant by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/26

## [v0.6.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.5.2...v0.6.0) - 2026-03-12
### Breaking Changes 🛠
- fix: normalize Copilot classification output by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/25
### Dependency Updates ⬆️
- chore(deps): bump github.com/github/copilot-sdk/go from 0.1.30 to 0.1.32 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/23

## [v0.5.2](https://github.com/k1LoW/gh-pr-reviews/compare/v0.5.1...v0.5.2) - 2026-03-06
### Fix bug 🐛
- feat: add OnPermissionRequest handler to Copilot session config by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/21

## [v0.5.1](https://github.com/k1LoW/gh-pr-reviews/compare/v0.5.0...v0.5.1) - 2026-03-06
### New Features 🎉
- feat: improve Copilot prompt to reduce unknown categories by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/20
### Dependency Updates ⬆️
- chore(deps): bump golang.org/x/term from 0.30.0 to 0.40.0 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/17
- chore(deps): bump github.com/github/copilot-sdk/go from 0.1.25 to 0.1.30 in the dependencies group by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/18
- chore(deps): bump the dependencies group with 2 updates by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/19

## [v0.5.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.4.0...v0.5.0) - 2026-02-18
### New Features 🎉
- fix: evaluate resolution status for question category via Copilot by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/15
### Other Changes
- docs: add Claude Code Skill example for triaging PR review comments by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/13

## [v0.4.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.3.0...v0.4.0) - 2026-02-18
### Breaking Changes 🛠
- feat: add colored Markdown output as default, move JSON to --json flag by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/8
- feat: change default copilot model to claude-haiku-4.5 by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/12
### Other Changes
- fix: improve markdown style by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/10
- feat: add shell completion for --copilot-model flag by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/11

## [v0.3.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.2.0...v0.3.0) - 2026-02-18
### New Features 🎉
- feat: add diff_hunk metadata to thread output by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/6

## [v0.2.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.1.0...v0.2.0) - 2026-02-18
### New Features 🎉
- feat: add path, line, and commit_id metadata to thread output by @k1LoW in https://github.com/k1LoW/gh-pr-reviews/pull/4

## [v0.1.0](https://github.com/k1LoW/gh-pr-reviews/compare/v0.0.1...v0.1.0) - 2026-02-18

## [v0.0.1](https://github.com/k1LoW/gh-pr-reviews/commits/v0.0.1) - 2026-02-18
### Dependency Updates ⬆️
- chore(deps): bump the dependencies group with 3 updates by @dependabot[bot] in https://github.com/k1LoW/gh-pr-reviews/pull/1
