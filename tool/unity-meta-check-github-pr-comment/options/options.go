package options

import (
	"flag"
	"fmt"
	"github.com/DeNA/unity-meta-check/options"
	"github.com/DeNA/unity-meta-check/tool/unity-meta-check-github-pr-comment/github"
	"github.com/DeNA/unity-meta-check/tool/unity-meta-check-github-pr-comment/l10n"
	"github.com/DeNA/unity-meta-check/tool/unity-meta-check-github-pr-comment/markdown"
	"github.com/DeNA/unity-meta-check/unity/checker"
	"github.com/DeNA/unity-meta-check/util/cli"
	"github.com/DeNA/unity-meta-check/util/logging"
	"github.com/DeNA/unity-meta-check/util/prefix"
	"github.com/DeNA/unity-meta-check/util/typedpath"
	"github.com/pkg/errors"
)

type Options struct {
	Version       bool
	LogLevel      logging.Severity
	Tmpl          *l10n.Template
	Token         github.Token
	Owner         github.Owner
	Repo          github.Repo
	PullNumber    github.PullNumber
	APIEndpoint   github.APIEndpoint
	SendIfSuccess bool
}

func BuildOptions(args []string, procInout cli.ProcessInout, env cli.Env) (*Options, error) {
	opts := &Options{}

	flags := flag.NewFlagSet("unity-meta-check-github-pr-comment", flag.ContinueOnError)
	flags.SetOutput(procInout.Stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintf(procInout.Stderr, `usage: unity-meta-check-github-pr-comment [<options>]

Post a comment for the result from unity-meta-check via STDIN to GitHub Pull Request.

OPTIONS
`)
		flags.PrintDefaults()

		_, _ = fmt.Fprintf(procInout.Stderr, `
ENVIRONMENT
  GITHUB_TOKEN
        GitHub API token. The scope can be empty if your repository is public. Otherwise, the scope should contain "repo"

EXAMPLE USAGES
  $ export GITHUB_TOKEN="********"
  $ unity-meta-check <options> | unity-meta-check-github-pr-comment \
      -api-endpoint https://api.github.com \
      -owner example-org \
      -repo my-repo \
      -pull "$CIRCLE_PR_NUMBER"  # This is for CircleCI

  $ export GITHUB_TOKEN="********"  # This should be set via credentials().
  $ unity-meta-check <options> | unity-meta-check-github-pr-comment \
      -api-endpoint https://github.example.com/api/v3 \
      -owner example-org \
      -repo my-repo \
      -pull "$ghprbPullId"  # This is for Jenkins with GitHub PullRequest Builder plugin

  $ GITHUB_TOKEN="********" unity-meta-check <options> | unity-meta-check-junit path/to/unity-meta-check-result.xml | unity-meta-check-github-pr-comment <options> | <other-unity-meta-check-tool>

  $ export GITHUB_TOKEN="********"  # This should be set via credentials().
  $ unity-meta-check <options> | unity-meta-check-github-pr-comment \
      -api-endpoint https://github.example.com/api/v3 \
      -owner example-org \
      -repo my-repo \
      -pull "$ghprbPullId" \
      -template-file path/to/template.json  # template file can be used for localization for GitHub comments.

TEMPLATE FILE FORMAT EXAMPLE
`)
		indentWriter := prefix.NewWriter("  ", procInout.Stderr)

		_, _ = fmt.Fprint(indentWriter, `If a template file is like:

`)
		l10n.WriteTemplateExample(indentWriter)
		_, _ = fmt.Fprint(indentWriter, `

then the output become:

`)
		_ = markdown.WriteMarkdown(&checker.CheckResult{
			MissingMeta:  []typedpath.SlashPath{},
			DanglingMeta: []typedpath.SlashPath{},
		}, &l10n.En, indentWriter)
		_, _ = fmt.Fprint(indentWriter, `
or:

`)
		_ = markdown.WriteMarkdown(&checker.CheckResult{
			MissingMeta:  []typedpath.SlashPath{"path/to/missing.meta"},
			DanglingMeta: []typedpath.SlashPath{"path/to/dangling.meta"},
		}, &l10n.En, indentWriter)

		_, _ = fmt.Fprintln(procInout.Stderr, "")
	}
	var unsafePullNumber int
	var debug, silent, noSendSuccess bool
	var lang, tmplPath, unsafeAPIEndpoint, unsafeOwner, unsafeRepo string
	flags.BoolVar(&opts.Version, "version", false, "print version")
	flags.BoolVar(&debug, "debug", false, "set log level to DEBUG (default INFO)")
	flags.BoolVar(&silent, "silent", false, "set log level to WARN (default INFO)")
	flags.StringVar(&lang, "lang", "en", "language code (available: en, ja)")
	flags.StringVar(&tmplPath, "template-file", "", "custom template file")
	flags.StringVar(&unsafeOwner, "owner", "", "owner of the GitHub repository")
	flags.StringVar(&unsafeRepo, "repo", "", "name of the GitHub repository")
	flags.IntVar(&unsafePullNumber, "pull", 0, "pull request number")
	flags.StringVar(&unsafeAPIEndpoint, "api-endpoint", "https://api.github.com", "GitHub API endpoint URL (like https://api.github.com or https://github.example.com/api/v3)")
	flags.BoolVar(&noSendSuccess, "no-send-success", false, "do not send a comment if no missing/dangling .meta found")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if opts.Version {
		return opts, nil
	}

	opts.LogLevel = cli.GetLogLevel(debug, silent)

	token, err := github.ValidateToken(env(options.GitHubTokenEnv))
	if err != nil {
		return nil, errors.Wrap(err, "invalid GITHUB_TOKEN: %s")
	}
	opts.Token = token

	owner, err := github.ValidateOwner(unsafeOwner)
	if err != nil {
		return nil, err
	}
	opts.Owner = owner

	repo, err := github.ValidateRepo(unsafeRepo)
	if err != nil {
		return nil, err
	}
	opts.Repo = repo

	pullNumber, err := github.ValidatePullNumber(unsafePullNumber)
	if err != nil {
		return nil, err
	}
	opts.PullNumber = pullNumber

	apiEndpoint, err := github.ValidateAPIEndpoint(unsafeAPIEndpoint)
	if err != nil {
		return nil, err
	}
	opts.APIEndpoint = apiEndpoint

	if tmplPath == "" {
		tmpl, err := l10n.GetTemplate(l10n.Lang(lang))
		if err != nil {
			return nil, err
		}
		opts.Tmpl = tmpl
	} else {
		tmpl, err := l10n.ReadTemplateFile(typedpath.NewRawPathUnsafe(tmplPath))
		if err != nil {
			return nil, err
		}
		if err := l10n.ValidateTemplate(tmpl); err != nil {
			return nil, err
		}
		opts.Tmpl = tmpl
	}

	opts.SendIfSuccess = !noSendSuccess

	return opts, nil
}
