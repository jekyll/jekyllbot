package chlog

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/google/go-github/v46/github"
	"github.com/jekyll/jekyllbot/auth"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/parkr/changelog"
)

const changeSectionLabelNone = "none"

// changelogCategory is a changelog category, like "Site Enhancements" and such.
type changelogCategory struct {
	Prefix, Slug, Section string
	Labels                []string
}

var (
	mergeCommentRegexp = regexp.MustCompile("@[a-zA-Z-_]+: (merge|:shipit:|:ship:)( \\+([a-zA-Z-_ ]+))?")
	mergeOptions       = &github.PullRequestOptions{MergeMethod: "squash"}

	categories = []changelogCategory{
		{
			Prefix:  "major",
			Slug:    "major-enhancements",
			Section: "Major Enhancements",
			Labels:  []string{"feature"},
		},
		{
			Prefix:  "minor",
			Slug:    "minor-enhancements",
			Section: "Minor Enhancements",
			Labels:  []string{"enhancement"},
		},
		{
			Prefix:  "bug",
			Slug:    "bug-fixes",
			Section: "Bug Fixes",
			Labels:  []string{"bug", "fix"},
		},
		{
			Prefix:  "fix",
			Slug:    "fix",
			Section: "Bug Fixes",
			Labels:  []string{"bug", "fix"},
		},
		{
			Prefix:  "dev",
			Slug:    "development-fixes",
			Section: "Development Fixes",
			Labels:  []string{"internal", "fix"},
		},
		{
			Prefix:  "doc",
			Slug:    "documentation",
			Section: "Documentation",
			Labels:  []string{"documentation"},
		},
		{
			Prefix:  "port",
			Slug:    "forward-ports",
			Section: "Forward Ports",
			Labels:  []string{"forward-port"},
		},
		{
			Prefix:  "site",
			Slug:    "site-enhancements",
			Section: "Site Enhancements",
			Labels:  []string{"documentation"},
		},
	}
)

type mergeAndLabelRequest struct {
	// Repo owner and repo name.
	Owner, Repo string
	// Pull request number.
	PullNumber int
	// Login of the user posting the comment
	CommenterLogin string
	// The changelog label in which to place the PR in the History/Changelog file
	ChangeSectionLabel string
}

func parseIssueCommentEvent(context *ctx.Context, event *github.IssueCommentEvent) (mergeAndLabelRequest, error) {
	req := &mergeAndLabelRequest{}

	// Is this a pull request?
	if event.Issue == nil || event.Issue.PullRequestLinks == nil {
		return *req, context.NewError("MergeAndLabel: comment not on a pull request")
	}

	req.Owner, req.Repo, req.PullNumber = *event.Repo.Owner.Login, *event.Repo.Name, *event.Issue.Number

	isReq, labelFromComment := parseMergeRequestComment(*event.Comment.Body)

	// Is It a merge request comment?
	if !isReq {
		return *req, context.NewError("MergeAndLabel: not a merge request comment")
	}

	// Should it be labeled?
	if labelFromComment != "" {
		req.ChangeSectionLabel = sectionForLabel(labelFromComment)
	} else {
		req.ChangeSectionLabel = changeSectionLabelNone
	}
	fmt.Printf("changeSectionLabel = '%s'\n", req.ChangeSectionLabel)

	req.CommenterLogin = *event.Comment.User.Login

	return *req, nil
}

func parsePullRequestReviewEvent(context *ctx.Context, event *github.PullRequestReviewEvent) (mergeAndLabelRequest, error) {
	req := &mergeAndLabelRequest{}

	if event.GetAction() != "submitted" {
		return *req, context.NewError("MergeAndLabel: review action is %q, not submitted", event.GetAction())
	}

	req.Owner, req.Repo, req.PullNumber = *event.Repo.Owner.Login, *event.Repo.Name, *event.PullRequest.Number

	req.CommenterLogin = *event.Review.User.Login

	isReq, labelFromComment := parseMergeRequestComment(*event.Review.Body)

	// Is It a merge request comment?
	if !isReq {
		return *req, context.NewError("MergeAndLabel: not a merge request review comment")
	}

	// Should it be labeled?
	if labelFromComment != "" {
		req.ChangeSectionLabel = sectionForLabel(labelFromComment)
	} else {
		req.ChangeSectionLabel = changeSectionLabelNone
	}
	fmt.Printf("changeSectionLabel = '%s'\n", req.ChangeSectionLabel)

	return *req, nil
}

func parseMergeAndLabelRequest(context *ctx.Context, payload interface{}) (mergeAndLabelRequest, error) {
	if event, ok := payload.(*github.IssueCommentEvent); ok {
		return parseIssueCommentEvent(context, event)
	}
	if event, ok := payload.(*github.PullRequestReviewEvent); ok {
		return parsePullRequestReviewEvent(context, event)
	}
	return mergeAndLabelRequest{}, context.NewError("MergeAndLabel: not an issue_comment or pull_request_review event")
}

func MergeAndLabel(context *ctx.Context, payload interface{}) error {
	req, err := parseMergeAndLabelRequest(context, payload)
	if err != nil {
		return err
	}

	if os.Getenv("AUTO_REPLY_DEBUG") == "true" {
		log.Println("MergeAndLabel: received event:", payload)
	}

	var wg sync.WaitGroup

	owner, repo, number := req.Owner, req.Repo, req.PullNumber
	ref := fmt.Sprintf("%s/%s#%d", owner, repo, number)

	// Does the user have merge/label abilities?
	if !auth.CommenterHasPushAccess(context, owner, repo, req.CommenterLogin) {
		log.Printf("%s isn't authenticated to merge anything on %s/%s", req.CommenterLogin, req.Owner, req.Repo)
		return errors.New("commenter isn't allowed to merge")
	}

	// Merge
	commitMsg := fmt.Sprintf("Merge pull request %v", number)
	_, _, mergeErr := context.GitHub.PullRequests.Merge(context.Context(), owner, repo, number, commitMsg, mergeOptions)
	if mergeErr != nil {
		return context.NewError("MergeAndLabel: error merging %s: %v", ref, mergeErr)
	}

	// Delete branch
	repoInfo, _, getRepoErr := context.GitHub.PullRequests.Get(context.Context(), owner, repo, number)
	if getRepoErr != nil {
		return context.NewError("MergeAndLabel: error getting PR info %s: %v", ref, getRepoErr)
	}

	if repoInfo == nil {
		return context.NewError("MergeAndLabel: tried to get PR, but couldn't. repoInfo was nil.")
	}

	// Delete branch
	if deletableRef(repoInfo, owner) {
		wg.Add(1)
		go func() {
			ref := fmt.Sprintf("heads/%s", *repoInfo.Head.Ref)
			_, deleteBranchErr := context.GitHub.Git.DeleteRef(context.Context(), owner, repo, ref)
			if deleteBranchErr != nil {
				fmt.Printf("MergeAndLabel: error deleting branch %v\n", mergeErr)
			}
			wg.Done()
		}()
	}

	wg.Add(1)
	go func() {
		err := addLabelsForSubsection(context, owner, repo, number, req.ChangeSectionLabel)
		if err != nil {
			fmt.Printf("MergeAndLabel: error applying labels: %v\n", err)
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		// Read History.markdown, add line to appropriate change section
		historyFileContents, historySHA := getHistoryContents(context, owner, repo)

		// Add merge reference to history
		newHistoryFileContents := addMergeReference(historyFileContents, req.ChangeSectionLabel, *repoInfo.Title, number)

		// Commit change to History.markdown
		commitErr := commitHistoryFile(context, historySHA, owner, repo, number, newHistoryFileContents)
		if commitErr != nil {
			fmt.Printf("comments: error committing updated history %v\n", mergeErr)
		}
		wg.Done()
	}()

	wg.Wait()

	return nil
}

func parseMergeRequestComment(commentBody string) (bool, string) {
	matches := mergeCommentRegexp.FindAllStringSubmatch(commentBody, -1)
	if matches == nil || matches[0] == nil {
		return false, ""
	}

	var label string
	if len(matches[0]) >= 4 {
		if labelFromComment := matches[0][3]; labelFromComment != "" {
			label = downcaseAndHyphenize(labelFromComment)
		}
	}

	return true, normalizeLabel(label)
}

func downcaseAndHyphenize(label string) string {
	return strings.Replace(strings.ToLower(label), " ", "-", -1)
}

func normalizeLabel(label string) string {
	for _, category := range categories {
		if strings.HasPrefix(label, category.Prefix) {
			return category.Slug
		}
	}

	return label
}

func sectionForLabel(slug string) string {
	for _, category := range categories {
		if slug == category.Slug {
			return category.Section
		}
	}

	return slug
}

func labelsForSubsection(changeSectionLabel string) []string {
	for _, category := range categories {
		if changeSectionLabel == category.Section {
			return category.Labels
		}
	}

	return []string{}
}

func selectSectionLabel(labels []github.Label) string {
	for _, label := range labels {
		if sectionForLabel(*label.Name) != *label.Name {
			return *label.Name
		}
	}
	return ""
}

func containsChangeLabel(commentBody string) bool {
	_, labelFromComment := parseMergeRequestComment(commentBody)
	return labelFromComment != ""
}

func addLabelsForSubsection(context *ctx.Context, owner, repo string, number int, changeSectionLabel string) error {
	labels := labelsForSubsection(changeSectionLabel)

	if len(labels) < 1 {
		return fmt.Errorf("no labels for changeSectionLabel='%s'", changeSectionLabel)
	}

	_, _, err := context.GitHub.Issues.AddLabelsToIssue(context.Context(), owner, repo, number, labels)
	return err
}

func getHistoryContents(context *ctx.Context, owner, repo string) (content, sha string) {
	defaultBranch := "master" // fallback
	repoInfo, _, err := context.GitHub.Repositories.Get(context.Context(), owner, repo)
	if err != nil {
		fmt.Printf("comments: error getting default branch for %s/%s", owner, repo)
	}
	if repoInfo.GetDefaultBranch() != "" {
		defaultBranch = repoInfo.GetDefaultBranch()
	}
	contents, _, _, err := context.GitHub.Repositories.GetContents(
		context.Context(),
		owner,
		repo,
		"History.markdown",
		&github.RepositoryContentGetOptions{Ref: "heads/" + defaultBranch},
	)
	if err != nil {
		fmt.Printf("comments: error getting History.markdown %v\n", err)
		return "", ""
	}
	return base64Decode(*contents.Content), *contents.SHA
}

func base64Decode(encoded string) string {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		fmt.Printf("comments: error decoding string: %v\n", err)
		return ""
	}
	return string(decoded)
}

func parseChangelog(historyFileContents string) (*changelog.Changelog, error) {
	changes, err := changelog.NewChangelogFromReader(strings.NewReader(historyFileContents))
	if historyFileContents == "" {
		err = nil
		changes = changelog.NewChangelog()
	}
	return changes, err
}

func addMergeReference(historyFileContents, changeSectionLabel, prTitle string, number int) string {
	changes, err := parseChangelog(historyFileContents)
	if err != nil {
		fmt.Printf("comments: error %v\n", err)
		return historyFileContents
	}

	changeLine := &changelog.ChangeLine{
		Summary:   template.HTMLEscapeString(prTitle),
		Reference: fmt.Sprintf("#%d", number),
	}

	// Put either directly in the version history or in a subsection.
	if changeSectionLabel == changeSectionLabelNone {
		changes.AddLineToVersion("HEAD", changeLine)
	} else {
		changes.AddLineToSubsection("HEAD", changeSectionLabel, changeLine)
	}

	return changes.String()
}

func deletableRef(pr *github.PullRequest, owner string) bool {
	return pr != nil &&
		pr.Head != nil &&
		pr.Head.Repo != nil &&
		pr.Head.Repo.Owner != nil &&
		pr.Head.Repo.Owner.Login != nil &&
		*pr.Head.Repo.Owner.Login == owner &&
		pr.Head.Ref != nil &&
		*pr.Head.Ref != "master" &&
		*pr.Head.Ref != "main" &&
		*pr.Head.Ref != "gh-pages"
}

func commitHistoryFile(context *ctx.Context, historySHA, owner, repo string, number int, newHistoryFileContents string) error {
	repositoryContentsOptions := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("Update history to reflect merge of #%d [ci skip]", number)),
		Content: []byte(newHistoryFileContents),
		SHA:     github.String(historySHA),
		Committer: &github.CommitAuthor{
			Name:  github.String("jekyllbot"),
			Email: github.String("jekyllbot@jekyllrb.com"),
		},
	}
	updateResponse, _, err := context.GitHub.Repositories.UpdateFile(context.Context(), owner, repo, "History.markdown", repositoryContentsOptions)
	if err != nil {
		fmt.Printf("comments: error committing History.markdown: %v\n", err)
		return err
	}
	fmt.Printf("comments: updateResponse: %s\n", updateResponse)
	return nil
}
