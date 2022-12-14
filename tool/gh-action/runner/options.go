package runner

import (
	"fmt"
	"github.com/DeNA/unity-meta-check/options"
	"github.com/DeNA/unity-meta-check/resultfilter"
	"github.com/DeNA/unity-meta-check/tool/gh-action/inputs"
	"github.com/DeNA/unity-meta-check/tool/unity-meta-autofix/autofix"
	prcomment "github.com/DeNA/unity-meta-check/tool/unity-meta-check-github-pr-comment/github"
	"github.com/DeNA/unity-meta-check/tool/unity-meta-check-github-pr-comment/l10n"
	"github.com/DeNA/unity-meta-check/unity/checker"
	"github.com/DeNA/unity-meta-check/util/globs"
	"github.com/DeNA/unity-meta-check/util/typedpath"
	"github.com/pkg/errors"
	"net/url"
	"strings"
)

type Options struct {
	RootDirAbs      typedpath.RawPath
	CheckerOpts     *checker.Options
	FilterOpts      *resultfilter.Options
	EnableJUnit     bool
	JUnitOutPath    typedpath.RawPath
	EnablePRComment bool
	PRCommentOpts   *prcomment.Options
	EnableAutofix   bool
	AutofixOpts     *autofix.Options
}

type Validator func(i inputs.Inputs, env inputs.ActionEnv) (*Options, error)

func NewValidateFunc(
	validateRootDirAbs options.RootDirAbsValidator,
	detectTargetType options.UnityProjectDetector,
	buildIgnoredGlobs options.IgnoredGlobsBuilder,
	buildAutofixOpts autofix.OptionsBuilder,
	readTmplFile l10n.TemplateFileReader,
	readEventPayload inputs.ReadEventPayloadFunc,
) Validator {
	return func(i inputs.Inputs, env inputs.ActionEnv) (*Options, error) {
		inputTargetType, err := inputs.ValidateTargetType(i.TargetType)
		if err != nil {
			return nil, err
		}

		rootDirAbs, err := validateRootDirAbs(i.TargetPath)
		if err != nil {
			return nil, err
		}

		var targetType checker.TargetType
		switch inputTargetType {
		case inputs.TargetTypeUnityProj:
			targetType = checker.TargetTypeIsUnityProjectRootDirectory
		case inputs.TargetTypeUnityProjSubDir, inputs.TargetTypeUpmPackage:
			targetType = checker.TargetTypeIsUnityProjectSubDirectory
		case inputs.TargetTypeAuto:
			detected, err := detectTargetType(rootDirAbs)
			if err != nil {
				return nil, err
			}
			targetType = detected
		default:
			return nil, fmt.Errorf("unknown target type: %q", inputTargetType)
		}

		checkerOpts := &checker.Options{
			IgnoreCase:                bool(i.IgnoreCase),
			IgnoreSubmodulesAndNested: bool(i.IgnoreSubmodulesAndNested),
			TargetType:                targetType,
		}

		ignoredGlobs, err := buildIgnoredGlobs(i.IgnoredFilePath, rootDirAbs)
		if err != nil {
			return nil, err
		}

		filterOpts := &resultfilter.Options{
			IgnoreDangling: bool(i.IgnoreDangling),
			IgnoredGlobs:   ignoredGlobs,
			IgnoreCase:     bool(i.IgnoreCase),
		}

		var junitPath typedpath.RawPath
		if i.EnableJUnit {
			junitPath = i.JUnitXMLPath
		}

		var prcommentOps *prcomment.Options
		if i.EnablePRComment {
			eventPayload, err := readEventPayload(env.EventPath)
			if err != nil {
				return nil, err
			}

			var pullNumber prcomment.PullNumber
			if i.PRCommentPullNumber > 0 {
				pullNumber = prcomment.PullNumber(i.PRCommentPullNumber)
			} else if eventPayload.PullRequest != nil {
				pullNumber = eventPayload.PullRequest.Number
			} else {
				return nil, fmt.Errorf("pull request comment can work only if triggered by pull request events or specified pr_comment_pull_number, but neither satisfied")
			}

			var tmpl *l10n.Template
			if i.PRCommentTmplFilePath == "" {
				tmpl, err = l10n.GetTemplate(l10n.Lang(i.PRCommentLang))
				if err != nil {
					return nil, err
				}
			} else {
				tmpl, err = readTmplFile(i.PRCommentTmplFilePath)
				if err != nil {
					return nil, err
				}
				if err := l10n.ValidateTemplate(tmpl); err != nil {
					return nil, err
				}
			}

			apiEndpoint, err := url.Parse(env.APIURL)
			if err != nil {
				return nil, errors.Wrapf(err, "malformed API endpoint URL: %q", env.APIURL)
			}

			prcommentOps = &prcomment.Options{
				Tmpl:          tmpl,
				SendIfSuccess: bool(i.PRCommentSendSuccess),
				Token:         env.GitHubToken,
				APIEndpoint:   apiEndpoint,
				Owner:         eventPayload.Repository.Owner.Login,
				Repo:          eventPayload.Repository.Name,
				PullNumber:    pullNumber,
			}
		}

		var autofixOpts *autofix.Options
		if i.EnableAutofix {
			autofixGlobs := strings.Split(i.CommaSeparatedAutofixGlobs, ",")
			allowedGlobs := make([]globs.Glob, len(autofixGlobs))
			for i, s := range autofixGlobs {
				allowedGlobs[i] = globs.Glob(s)
			}
			autofixOpts, err = buildAutofixOpts(rootDirAbs, allowedGlobs)
			if err != nil {
				return nil, err
			}
		}

		return &Options{
			RootDirAbs:      rootDirAbs,
			CheckerOpts:     checkerOpts,
			FilterOpts:      filterOpts,
			EnableJUnit:     bool(i.EnableJUnit),
			JUnitOutPath:    junitPath,
			EnablePRComment: bool(i.EnablePRComment),
			PRCommentOpts:   prcommentOps,
			EnableAutofix:   bool(i.EnableAutofix),
			AutofixOpts:     autofixOpts,
		}, nil
	}
}
