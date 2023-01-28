// Package update-dot-github creates PRs to update .github files when they're out of date.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/jekyll/jekyllbot/ctx"
	"github.com/jekyll/jekyllbot/jekyll"
	"github.com/jekyll/jekyllbot/sentry"
	"golang.org/x/sync/errgroup"
)

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
		return jekyll.DefaultRepos // TODO: filter out jekyll/jekyll.
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
	log.Printf("Processing repo %s", repo.String())
	// 1. Check on Dependabot:
	// 		Does it exist?
	//		Does it have the correct contents?
	dependabotMatch, err := checkContentsMatch(context, expectedDependabotContents, repo, ".github/dependabot.yml")
	if err != nil {
		log.Printf("[%s] unable to check dependabot config: %+v", repo.String(), err)
	} else if !dependabotMatch {
		if perform {
			if err := proposeChanges(context, repo, expectedCIWorkflowContents, ".github/dependabot.yml"); err != nil {
				log.Printf("[%s] error updating dependabot config: %+v", repo.String(), err)
			}
			proposeChanges(context, repo, expectedDependabotContents, ".github/dependabot.yml")
		} else {
			log.Printf("[%s] skipping updating dependabot config", repo.String())
		}
	}
	// 2. Check on .github/workflows/ci.yaml
	//      Does it have the correct contents?
	ciWorkflowMatch, err := checkContentsMatch(context, expectedCIWorkflowContents, repo, ".github/workflows/ci.yaml")
	if err != nil {
		log.Printf("[%s] unable to check ci workflow: %+v", repo.String(), err)
	} else if !ciWorkflowMatch {
		if perform {
			if err := proposeChanges(context, repo, expectedCIWorkflowContents, ".github/workflows/ci.yml"); err != nil {
				log.Printf("[%s] error updating ci workflow: %+v", repo.String(), err)
			}
		} else {
			log.Printf("[%s] skipping updating ci workflow", repo.String())
		}
	}
	// 3. Check on .github/workflows/release.yaml
	// 		Does it have the correct contents?
	releaseWorkflowMatch, err := checkContentsMatch(context, expectedReleaseWorkflowContents, repo, ".github/workflows/release.yaml")
	if err != nil {
		log.Printf("[%s] unable to check release workflow: %+v", repo.String(), err)
	} else if !releaseWorkflowMatch {
		if perform {
			if err := proposeChanges(context, repo, expectedReleaseWorkflowContents, ".github/release.yml"); err != nil {
				log.Printf("[%s] error updating release workflow: %+v", repo.String(), err)
			}
		} else {
			log.Printf("[%s] skipping updating release workflow", repo.String())
		}
	}
	return nil
}

func checkContentsMatch(context *ctx.Context, expectedContents string, repo jekyll.Repository, filepath string) (bool, error) {
	fileContent, _, resp, err := context.GitHub.Repositories.GetContents(context.Context(), repo.Owner(), repo.Name(), filepath, nil)
	if resp.Response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	actualContents, err := fileContent.GetContent()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(expectedContents) == strings.TrimSpace(actualContents), nil
}

func proposeChanges(context *ctx.Context, repo jekyll.Repository, newContents string, filepath string) error {
	// 1. Create branch
	// 2. Write contents to new branch
	// 3. Create pull request
	return errors.New("unimplemented")
}

var expectedDependabotContents = `# Dependabot automatically keeps our packages up to date
# Docs: https://docs.github.com/en/free-pro-team@latest/github/administering-a-repository/configuration-options-for-dependency-updates

version: 2
updates:
- package-ecosystem: bundler
  directory: "/"
  schedule:
    interval: daily
    time: "11:00"
  open-pull-requests-limit: 99
  reviewers:
  - jekyll/plugin-core
- package-ecosystem: github-actions
  directory: "/"
  schedule:
    interval: daily
    time: "11:00"
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
    - /.*-stable/
  pull_request:
    branches:
    - main
    - /.*-stable/

jobs:
  ci:
    if: "!contains(github.event.commits[0].message, '[ci skip]')"
    name: 'Ruby ${{ matrix.ruby_version }} ${{ matrix.os }}'
    runs-on: '${{ matrix.os }}'
    strategy:
      fail-fast: false
      matrix:
        ruby_version:
        - 2.7
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

var expectedReleaseWorkflowContents = `name: Release Gem

on:
  push:
    branches:
      - main
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
          - 2.7
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
