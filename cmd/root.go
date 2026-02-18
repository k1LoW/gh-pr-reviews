/*
Copyright © 2026 Ken'ichiro Oyama <k1lowxb@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/mattn/go-colorable"
	"github.com/muesli/termenv"

	"github.com/k1LoW/gh-pr-reviews/gh"
	"github.com/k1LoW/gh-pr-reviews/output"
	"github.com/k1LoW/gh-pr-reviews/review"
	"github.com/k1LoW/gh-pr-reviews/version"
	"github.com/spf13/cobra"
)

var (
	flagRepoSelector string
	showAll          bool
	copilotModel     string
	verbose          bool
	jsonOutput       bool
	widthFlag        int
)

var rootCmd = &cobra.Command{
	Use:     "gh-pr-reviews [<pr-number> | <pr-url> | <branch>]",
	Short:   "Show unresolved review comments for a pull request",
	Long:    `gh-pr-reviews identifies unresolved review comments in a pull request using Copilot to classify and determine resolution status.`,
	Version: version.Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		level := slog.LevelError
		if verbose {
			level = slog.LevelInfo
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(colorable.NewColorableStderr()))
		_ = s.Color("fgHiMagenta")

		// Resolve PR context via gh CLI.
		s.Suffix = " Resolving PR..."
		s.Start()
		prInfo, err := resolvePR(args, flagRepoSelector)
		if err != nil {
			s.Stop()
			return err
		}
		slog.Info("resolved PR", "owner", prInfo.owner, "repo", prInfo.repo, "number", prInfo.number)

		// Create GitHub GraphQL client.
		ghClient, err := gh.New()
		if err != nil {
			s.Stop()
			return err
		}

		// Fetch review data.
		s.Suffix = " Fetching review data..."
		data, err := ghClient.FetchReviews(ctx, prInfo.owner, prInfo.repo, prInfo.number)
		if err != nil {
			s.Stop()
			return err
		}
		slog.Info("fetched review data", "threads", len(data.Threads), "pr_comments", len(data.PRComments))

		// Create Copilot classifier.
		s.Suffix = " Starting Copilot..."
		classifier, err := review.NewCopilotClassifier(ctx, copilotModel)
		if err != nil {
			s.Stop()
			return fmt.Errorf("failed to create classifier: %w", err)
		}
		defer classifier.Close()

		// Analyze reviews.
		s.Suffix = " Classifying review comments..."
		results, err := review.Analyze(ctx, data, classifier, showAll)
		s.Stop()
		if err != nil {
			return err
		}

		if jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}
		} else {
			p := termenv.NewOutput(os.Stdout, termenv.WithColorCache(true))
			w := output.DetectWidth(widthFlag)
			output.RenderMarkdown(os.Stdout, results, p, w)
		}

		return nil
	},
}

type prContext struct {
	owner  string
	repo   string
	number int
}

func resolvePR(args []string, repoSelector string) (*prContext, error) {
	ghArgs := []string{"pr", "view", "--json", "number,url,headRepository,headRepositoryOwner"}
	if len(args) > 0 {
		ghArgs = append(ghArgs[:2], append([]string{args[0]}, ghArgs[2:]...)...)
	}
	if repoSelector != "" {
		ghArgs = append(ghArgs, "--repo", repoSelector)
	}

	out, err := exec.Command("gh", ghArgs...).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("gh pr view failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}

	var result struct {
		Number              int    `json:"number"`
		URL                 string `json:"url"`
		HeadRepository      struct{ Name string } `json:"headRepository"`
		HeadRepositoryOwner struct{ Login string } `json:"headRepositoryOwner"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr view output: %w", err)
	}

	if result.Number == 0 {
		return nil, fmt.Errorf("could not determine PR number from gh pr view output")
	}

	owner := result.HeadRepositoryOwner.Login
	repo := result.HeadRepository.Name

	// Fallback: parse from URL if needed.
	if owner == "" || repo == "" {
		parts := strings.Split(strings.TrimSuffix(result.URL, "/"), "/")
		if len(parts) >= 5 {
			if owner == "" {
				owner = parts[len(parts)-4]
			}
			if repo == "" {
				repo = parts[len(parts)-3]
			}
		}
	}

	if owner == "" || repo == "" {
		return nil, fmt.Errorf("could not determine repository owner/name")
	}

	return &prContext{
		owner:  owner,
		repo:   repo,
		number: result.Number,
	}, nil
}

// Execute runs the root command.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&flagRepoSelector, "repo", "R", "", "Select another repository using the [HOST/]OWNER/REPO format")
	rootCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all review comments including resolved ones")
	rootCmd.Flags().StringVar(&copilotModel, "copilot-model", "gpt-4o", "Copilot model to use for classification")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose output")
	rootCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output results as JSON")
	rootCmd.Flags().IntVarP(&widthFlag, "width", "w", 0, "Output width (0 for auto-detect)")

	// Hide the default completion command.
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Use PR number as arg for convenience.
	rootCmd.Args = cobra.MaximumNArgs(1)
	rootCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Suppress usage on errors from RunE.
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
}

