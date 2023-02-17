// A command-line utility to lock old issues.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/google/go-github/v50/github"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/jekyll/jekyllbot/freeze"
	"github.com/jekyll/jekyllbot/jekyll"
	"github.com/jekyll/jekyllbot/sentry"
)

var (
	defaultRepos = jekyll.DefaultRepos

	sleepBetweenFreezes = 150 * time.Millisecond
)

func main() {
	var actuallyDoIt bool
	flag.BoolVar(&actuallyDoIt, "f", false, "Whether to actually mark the issues or close them.")
	var inputRepos string
	flag.StringVar(&inputRepos, "repos", "", "Specify a list of comma-separated repo name/owner pairs, e.g. 'jekyll/jekyll-import'.")
	flag.Parse()

	log.SetPrefix("freeze-ancient-issues: ")

	var repos []jekyll.Repository
	if inputRepos == "" {
		repos = defaultRepos
	} else {
		for _, repoNWO := range strings.Split(inputRepos, ",") {
			repo, err := jekyll.ParseRepository(repoNWO)
			if err != nil {
				repos = append(repos, repo)
			} else {
				log.Println(err)
			}
		}
	}

	sentryClient, err := sentry.NewClient(map[string]string{
		"app":          "freeze-ancient-issues",
		"inputRepos":   inputRepos,
		"actuallyDoIt": fmt.Sprintf("%t", actuallyDoIt),
	})
	if err != nil {
		panic(err)
	}
	sentryClient.Recover(func() error {
		context := ctx.NewDefaultContext()
		if context.GitHub == nil {
			return errors.New("cannot proceed without github client")
		}

		// Support running on just a list of issues. Either a URL or a `owner/name#number` syntax.
		if flag.NArg() > 0 {
			return processSingleIssues(context, actuallyDoIt, flag.Args()...)
		}

		wg, _ := errgroup.WithContext(context.Context())
		for _, repo := range repos {
			repo := repo
			wg.Go(func() error {
				return processRepo(context, repo.Owner(), repo.Name(), actuallyDoIt)
			})
		}

		return wg.Wait()
	})
}

func extractIssueInfo(issueName string) (owner, repo string, number int) {
	issueName = strings.TrimPrefix(issueName, "https://github.com/")

	var err error
	pieces := strings.Split(issueName, "/")

	// Ex: `owner/repo#number`
	if len(pieces) == 2 {
		owner = pieces[0]
		morePieces := strings.Split(pieces[1], "#")
		if len(morePieces) == 2 {
			repo = morePieces[0]
			number, err = strconv.Atoi(morePieces[1])
			if err != nil {
				log.Printf("huh? %#v for %s", err, morePieces[1])
			}
		}
		return
	}

	// Ex: `owner/repo/issues/number`
	if len(pieces) == 4 {
		owner = pieces[0]
		repo = pieces[1]
		number, err = strconv.Atoi(pieces[3])
		if err != nil {
			log.Printf("huh? %#v for %s", err, pieces[3])
		}
		return
	}

	return "", "", 0
}

func processSingleIssues(context *ctx.Context, actuallyDoIt bool, issueNames ...string) error {
	issues := []github.Issue{}
	for _, issueName := range issueNames {
		owner, repo, number := extractIssueInfo(issueName)
		if owner == "" || repo == "" || number <= 0 {
			return fmt.Errorf("couldn't extract issue info from '%s': owner=%s repo=%s number=%d",
				issueName, owner, repo, number)
		}

		issues = append(issues, github.Issue{
			Number: github.Int(number),
			Repository: &github.Repository{
				Owner: &github.User{Login: github.String(owner)},
				Name:  github.String(repo),
			},
		})
	}
	return processIssues(context, actuallyDoIt, issues)
}

func processRepo(context *ctx.Context, owner, repo string, actuallyDoIt bool) error {
	start := time.Now()

	issues, err := freeze.AllTooOldIssues(context, owner, repo)
	if err != nil {
		return err
	}

	log.Printf("%s/%s: freezing %d closed issues before %v", owner, repo, len(issues), freeze.TooOld)
	err = processIssues(context, actuallyDoIt, issues)
	log.Printf("%s/%s: finished in %s", owner, repo, time.Since(start))

	return err
}

func processIssues(context *ctx.Context, actuallyDoIt bool, issues []github.Issue) error {
	for _, issue := range issues {
		context.Log("processing issue: %#v", issue)
		repo, err := jekyll.ParseRepositoryFromURL(issue.GetHTMLURL())
		if err != nil {
			return err
		}
		if actuallyDoIt {
			log.Printf("%s/%s: freezing %s", repo.Owner(), repo.Name(), issue.GetHTMLURL())
			if err := freeze.Freeze(context, repo.Owner(), repo.Name(), issue.GetNumber()); err != nil {
				return err
			}
			time.Sleep(sleepBetweenFreezes)
		} else {
			log.Printf("%s/%s: would have frozen %s", repo.Owner(), repo.Name(), issue.GetHTMLURL())
			time.Sleep(sleepBetweenFreezes)
		}
	}
	return nil
}
