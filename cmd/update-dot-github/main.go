// Package update-dot-github creates PRs to update .github files when they're out of date.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/go-github/v50/github"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/jekyll/jekyllbot/jekyll"
	"github.com/jekyll/jekyllbot/sentry"
	"golang.org/x/sync/errgroup"
)

const nofilefound404 = "404meansnofile"

func main() {
	var perform bool
	flag.BoolVar(&perform, "doit", false, "Whether to actually file PRs.")
	var inputRepos string
	flag.StringVar(&inputRepos, "repos", "", "Specify a list of comma-separated repo name/owner pairs, e.g. 'jekyll/jekyll-import'.")
	var enableSentry bool
	flag.BoolVar(&enableSentry, "sentry", true, "Enable or disable Sentry error integration")
	flag.Parse()

	log.SetPrefix("update-dot-github: ")

	if enableSentry {
		sentryClient, err := sentry.NewClient(map[string]string{
			"app":          "update-dot-github",
			"inputRepos":   inputRepos,
			"actuallyDoIt": fmt.Sprintf("%t", perform),
		})
		if err != nil {
			panic(err)
		}
		sentryClient.Recover(func() error {
			return updateDotGitHub(perform, parseCSVReposOrDefault(inputRepos))
		})
	} else {
		updateDotGitHub(perform, parseCSVReposOrDefault(inputRepos))
	}
}

func updateDotGitHub(perform bool, repos []jekyll.Repository) error {
	context := ctx.NewDefaultContext()
	if context.GitHub == nil {
		return errors.New("cannot proceed without github client")
	}
	log.Printf("authenticated user is %s", context.CurrentlyAuthedGitHubUser().GetLogin())

	// TODO: perhaps I need to use this derived context
	wg, _ := errgroup.WithContext(context.Context())
	for _, repo := range repos {
		repo := repo
		wg.Go(func() error {
			return processRepo(context, perform, repo)
		})
	}
	return wg.Wait()
}

func parseCSVReposOrDefault(inputRepos string) []jekyll.Repository {
	if inputRepos == "" {
		return jekyll.DefaultRepos
	}

	repos := []jekyll.Repository{}
	for _, inputRepo := range strings.Split(inputRepos, ",") {
		pieces := strings.Split(inputRepo, "/")
		if len(pieces) != 2 {
			log.Printf("WARN: input repo %q is improperly formed", inputRepo)
			continue
		}
		repos = append(repos, jekyll.NewRepository(pieces[0], pieces[1]))
	}
	return repos
}

func processRepo(context *ctx.Context, perform bool, repo jekyll.Repository) error {
	if repo.Name() == "jekyll" {
		return errors.New("error: cannot generate for jekyll/jekyll")
	}
	log.Printf("Processing repo %s", repo.String())

	err := ensureContentsAreAsExpected(context, repo, perform, ".github/dependabot.yml", expectedDependabotContents)
	if err != nil {
		log.Printf("[%s] %+v", repo.String(), err)
	}

	err = ensureContentsAreAsExpected(context, repo, perform, ".github/workflows/ci.yaml", expectedCIWorkflowContents)
	if err != nil {
		log.Printf("[%s] %+v", repo.String(), err)
	}

	expectedReleaseWorkflowContents, err := generateReleaseWorkflowContents(repo)
	if err != nil {
		return err
	}
	err = ensureContentsAreAsExpected(context, repo, perform, ".github/workflows/release.yaml", expectedReleaseWorkflowContents)
	if err != nil {
		log.Printf("[%s] %+v", repo.String(), err)
	}

	return err
}

func ensureContentsAreAsExpected(context *ctx.Context, repo jekyll.Repository, perform bool, filepath, expectedContents string) error {
	doContentsMatch, filepathLatestSHA, err := checkContentsMatch(context, expectedContents, repo, filepath)
	if err != nil {
		return err
	} else if !doContentsMatch {
		if perform {
			return proposeChanges(context, repo, filepathLatestSHA, expectedContents, filepath)
		} else {
			return fmt.Errorf("skipping update of stale file %s", filepath)
		}
	}
	return nil
}

func checkContentsMatch(context *ctx.Context, expectedContents string, repo jekyll.Repository, filepath string) (bool, string, error) {
	fileContent, _, resp, err := context.GitHub.Repositories.GetContents(context.Context(), repo.Owner(), repo.Name(), filepath, nil)
	if resp.Response.StatusCode == http.StatusNotFound {
		return false, nofilefound404, nil
	}
	if err != nil {
		return false, "", err
	}
	actualContents, err := fileContent.GetContent()
	if err != nil {
		return false, "", err
	}
	return strings.TrimSpace(expectedContents) == strings.TrimSpace(actualContents), fileContent.GetSHA(), nil
}

func proposeChanges(context *ctx.Context, repo jekyll.Repository, existingSHA, newContents, filePath string) error {
	// -1. Get default branch.
	defaultBranch := "master" // fallback
	repoInfo, _, err := context.GitHub.Repositories.Get(context.Context(), repo.Owner(), repo.Name())
	if err != nil {
		return err
	}
	if repoInfo.GetDefaultBranch() != "" {
		defaultBranch = repoInfo.GetDefaultBranch()
	}

	// 0. Get latest SHA for HEAD
	headRef, _, err := context.GitHub.Git.GetRef(context.Context(), repo.Owner(), repo.Name(), "heads/"+defaultBranch)
	if err != nil {
		return err
	}

	// 1. Create branch using a random input
	branchName := fmt.Sprintf("update-dot-github-file-%s", strings.ReplaceAll(filePath, "/", "-"))
	ref := &github.Reference{
		Ref:    github.String("refs/heads/" + branchName),
		Object: headRef.GetObject(),
	}
	_, _, err = context.GitHub.Git.CreateRef(context.Context(), repo.Owner(), repo.Name(), ref)
	if err != nil {
		return err
	}

	// 2. Write contents to new branch
	repositoryContentsOptions := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Update %s", filePath)),
		Content: []byte(newContents),
		Branch:  github.String(branchName),
		Committer: &github.CommitAuthor{
			Name:  github.String("jekyllbot"),
			Email: github.String("jekyllbot@jekyllrb.com"),
		},
	}
	if existingSHA == nofilefound404 {
		_, _, err := context.GitHub.Repositories.CreateFile(context.Context(), repo.Owner(), repo.Name(), filePath, repositoryContentsOptions)
		if err != nil {
			return err
		}
	} else {
		repositoryContentsOptions.SHA = github.String(existingSHA)
		_, _, err := context.GitHub.Repositories.UpdateFile(context.Context(), repo.Owner(), repo.Name(), filePath, repositoryContentsOptions)
		if err != nil {
			return err
		}
	}

	// 3. Create pull request
	pull := &github.NewPullRequest{
		Title:               github.String(fmt.Sprintf("Update %s", filePath)),
		Head:                github.String(branchName),
		Base:                github.String(defaultBranch),
		Body:                github.String(fmt.Sprintf(prBodyTmpl, filePath)),
		MaintainerCanModify: github.Bool(true),
	}
	pr, _, err := context.GitHub.PullRequests.Create(context.Context(), repo.Owner(), repo.Name(), pull)
	log.Printf("[%s] filed PR: %s", repo.String(), pr.GetHTMLURL())

	return err
}

func generateReleaseWorkflowContents(repo jekyll.Repository) (string, error) {
	if repo.GemspecName() == "" {
		return "", errors.New("unable to generate release workflow for non-gem repo")
	}
	return fmt.Sprintf(expectedReleaseWorkflowContentsTmpl, repo.GemspecName()), nil
}

var expectedDependabotContents = `---
# Dependabot automatically keeps our packages up to date
# Docs: https://docs.github.com/en/free-pro-team@latest/github/administering-a-repository/configuration-options-for-dependency-updates

version: 2
updates:
- package-ecosystem: bundler
  directory: "/"
  schedule:
    interval: daily
  open-pull-requests-limit: 99
  reviewers:
  - jekyll/plugin-core
- package-ecosystem: github-actions
  directory: "/"
  schedule:
    interval: daily
  open-pull-requests-limit: 99
  reviewers:
  - jekyll/plugin-core
`

var expectedCIWorkflowContents = `---
name: Continuous Integration

on:
  push:
    branches:
    - main
    - master
    - ".*-stable"
  pull_request:
    branches:
    - main
    - master
    - ".*-stable"

jobs:
  ci:
    if: "!contains(github.event.commits[0].message, '[ci skip]')"
    name: 'Ruby ${{ matrix.ruby_version }} ${{ matrix.os }}'
    runs-on: '${{ matrix.os }}'
    strategy:
      fail-fast: false
      matrix:
        ruby_version:
        - '2.7'
        - '3.0'
        - '3.1'
        - '3.2'
        os:
        - ubuntu-latest
        - windows-latest
    steps:
      - uses: actions/checkout@v3
      - uses: ruby/setup-ruby@v1
        with:
          ruby-version: ${{ matrix.ruby_version }}
          bundler-cache: true # runs 'bundle install' and caches installed gems automatically
      - run: script/cibuild
`

var expectedReleaseWorkflowContentsTmpl = `---
name: Release Gem

on:
  push:
    branches:
      - main
      - master
      - ".*-stable"
    paths:
	  - "lib/**/version.rb"

jobs:
  release:
    if: "github.repository_owner == 'jekyll'"
    name: "Release Gem (Ruby ${{ matrix.ruby_version }})"
    runs-on: "ubuntu-latest"
    strategy:
      fail-fast: true
      matrix:
        ruby_version:
          - "2.7"
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v3
      - name: "Set up Ruby ${{ matrix.ruby_version }}"
        uses: ruby/setup-ruby@v1
        with:
          ruby-version: ${{ matrix.ruby_version }}
          bundler-cache: true
      - name: Build and Publish Gem
        uses: ashmaroli/release-gem@dist
        with:
          gemspec_name: %s
        env:
          GEM_HOST_API_KEY: ${{ secrets.RUBYGEMS_GEM_PUSH_API_KEY }}
`

var prBodyTmpl = `Hey @jekyll/plugin-core!

There's been an update to the ` + "`%s`" + ` file template in jekyll/jekyllbot. This PR should bring this repo up to date.

Thanks! :revolving_hearts: :sparkles: :robot:
`
