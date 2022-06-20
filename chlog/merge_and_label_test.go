package chlog

import (
	"io/ioutil"
	"testing"

	"github.com/google/go-github/github"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/stretchr/testify/assert"
)

func TestParseMergeAndLabelRequest(t *testing.T) {
	context := ctx.NewTestContext()
	var payload interface{}
	payload = &github.IssueCommentEvent{}

	_, err := parseMergeAndLabelRequest(context, payload)
	assert.EqualError(t, err, "MergeAndLabel: comment not on a pull request")

	payload = &github.PullRequestReviewEvent{}
	_, err = parseMergeAndLabelRequest(context, payload)
	assert.EqualError(t, err, "MergeAndLabel: review action is \"\", not submitted")

	payload = &github.PullRequestReviewCommentEvent{}
	_, err = parseMergeAndLabelRequest(context, payload)
	assert.EqualError(t, err, "MergeAndLabel: not an issue_comment or pull_request_review event")
}

func TestParsePullRequestReviewEvent_NotMergeComment(t *testing.T) {
	context := ctx.NewTestContext()
	event := &github.PullRequestReviewEvent{
		Action: github.String("submitted"),
		PullRequest: &github.PullRequest{
			Number: github.Int(456),
		},
		Review: &github.PullRequestReview{
			User: &github.User{
				Login: github.String("monalisa"),
			},
			Body: github.String("well done!"),
		},
		Repo: &github.Repository{
			Owner: &github.User{Login: github.String("owner-login")},
			Name:  github.String("foo"),
		},
	}
	_, err := parsePullRequestReviewEvent(context, event)
	assert.EqualError(t, err, "MergeAndLabel: not a merge request review comment")
}

func TestParsePullRequestReviewEvent_NoChangelogLabel(t *testing.T) {
	context := ctx.NewTestContext()
	event := &github.PullRequestReviewEvent{
		Action: github.String("submitted"),
		PullRequest: &github.PullRequest{
			Number: github.Int(456),
		},
		Review: &github.PullRequestReview{
			User: &github.User{
				Login: github.String("monalisa"),
			},
			Body: github.String("@jekyllbot: merge"),
		},
		Repo: &github.Repository{
			Owner: &github.User{Login: github.String("owner-login")},
			Name:  github.String("foo"),
		},
	}
	req, err := parsePullRequestReviewEvent(context, event)
	assert.NoError(t, err)

	assert.Equal(t, "owner-login", req.Owner)
	assert.Equal(t, "foo", req.Repo)
	assert.Equal(t, 456, req.PullNumber)
	assert.Equal(t, "monalisa", req.CommenterLogin)
	assert.Equal(t, changeSectionLabelNone, req.ChangeSectionLabel)
}

func TestParsePullRequestReviewEvent_Works(t *testing.T) {
	context := ctx.NewTestContext()
	event := &github.PullRequestReviewEvent{
		Action: github.String("submitted"),
		PullRequest: &github.PullRequest{
			Number: github.Int(456),
		},
		Review: &github.PullRequestReview{
			User: &github.User{
				Login: github.String("monalisa"),
			},
			Body: github.String("@jekyllbot: merge +fix"),
		},
		Repo: &github.Repository{
			Owner: &github.User{Login: github.String("owner-login")},
			Name:  github.String("foo"),
		},
	}
	req, err := parsePullRequestReviewEvent(context, event)
	assert.NoError(t, err)

	assert.Equal(t, "owner-login", req.Owner)
	assert.Equal(t, "foo", req.Repo)
	assert.Equal(t, 456, req.PullNumber)
	assert.Equal(t, "monalisa", req.CommenterLogin)
	assert.Equal(t, "Bug Fixes", req.ChangeSectionLabel)
}
func TestParseIssueCommentEvent_NotPullRequest(t *testing.T) {
	context := ctx.NewTestContext()
	event := &github.IssueCommentEvent{
		Issue: &github.Issue{},
	}
	_, err := parseIssueCommentEvent(context, event)
	assert.EqualError(t, err, "MergeAndLabel: comment not on a pull request")
}

func TestParseIssueCommentEvent_NotMergeComment(t *testing.T) {
	context := ctx.NewTestContext()
	event := &github.IssueCommentEvent{
		Issue: &github.Issue{
			Number:           github.Int(456),
			PullRequestLinks: &github.PullRequestLinks{},
		},
		Repo: &github.Repository{
			Owner: &github.User{Login: github.String("owner-login")},
			Name:  github.String("foo"),
		},
		Comment: &github.IssueComment{
			Body: github.String("howdy"),
		},
	}
	_, err := parseIssueCommentEvent(context, event)
	assert.EqualError(t, err, "MergeAndLabel: not a merge request comment")
}

func TestParseIssueCommentEvent_NoChangelogLabel(t *testing.T) {
	context := ctx.NewTestContext()
	event := &github.IssueCommentEvent{
		Issue: &github.Issue{
			Number:           github.Int(456),
			PullRequestLinks: &github.PullRequestLinks{},
		},
		Repo: &github.Repository{
			Owner: &github.User{Login: github.String("owner-login")},
			Name:  github.String("foo"),
		},
		Comment: &github.IssueComment{
			User: &github.User{Login: github.String("monalisa")},
			Body: github.String("@jekyllbot: merge"),
		},
	}
	req, err := parseIssueCommentEvent(context, event)
	assert.NoError(t, err)

	assert.Equal(t, "owner-login", req.Owner)
	assert.Equal(t, "foo", req.Repo)
	assert.Equal(t, 456, req.PullNumber)
	assert.Equal(t, "monalisa", req.CommenterLogin)
	assert.Equal(t, changeSectionLabelNone, req.ChangeSectionLabel)
}

func TestParseIssueCommentEvent_Works(t *testing.T) {
	context := ctx.NewTestContext()
	event := &github.IssueCommentEvent{
		Issue: &github.Issue{
			Number:           github.Int(456),
			PullRequestLinks: &github.PullRequestLinks{},
		},
		Repo: &github.Repository{
			Owner: &github.User{Login: github.String("owner-login")},
			Name:  github.String("foo"),
		},
		Comment: &github.IssueComment{
			User: &github.User{Login: github.String("monalisa")},
			Body: github.String("@jekyllbot: merge +fix"),
		},
	}
	req, err := parseIssueCommentEvent(context, event)
	assert.NoError(t, err)

	assert.Equal(t, "owner-login", req.Owner)
	assert.Equal(t, "foo", req.Repo)
	assert.Equal(t, 456, req.PullNumber)
	assert.Equal(t, "Bug Fixes", req.ChangeSectionLabel)
}

func TestParseMergeRequestComment(t *testing.T) {
	comments := []struct {
		comment string
		isReq   bool
		label   string
		section string
		labels  []string
	}{
		{"it looked like you could merge it", false, "", "", []string{}},
		{"@jekyllbot: merge", true, "", "", []string{}},
		{"@jekyllbot: :shipit:", true, "", "", []string{}},
		{"@jekyllbot: :ship:", true, "", "", []string{}},
		{"@jekyllbot: merge +Site", true, "site-enhancements", "Site Enhancements", []string{"documentation"}},
		{"@jekyllbot: merge +major", true, "major-enhancements", "Major Enhancements", []string{"feature"}},
		{"@jekyllbot: merge +minor-enhancement", true, "minor-enhancements", "Minor Enhancements", []string{"enhancement"}},
		{"@jekyllbot: merge +Bug Fix\n", true, "bug-fixes", "Bug Fixes", []string{"bug", "fix"}},
		{"@jekyllbot: merge +port", true, "forward-ports", "Forward Ports", []string{"forward-port"}},
	}
	for _, c := range comments {
		isReq, label := parseMergeRequestComment(c.comment)
		section := sectionForLabel(c.label)
		assert.Equal(t, c.isReq, isReq, "'%s' should have isReq=%v", c.comment, c.isReq)
		assert.Equal(t, c.label, label, "'%s' should have label=%v", c.comment, c.label)
		assert.Equal(t, c.section, section, "'%s' should have section=%v", c.comment, c.section)
		assert.Equal(t, c.labels, labelsForSubsection(section), "'%s' should have labels=%v", c.comment, c.labels)
	}
}

func TestBase64Decode(t *testing.T) {
	encoded, err := ioutil.ReadFile("history_contents.enc")
	assert.NoError(t, err)
	decoded := base64Decode(string(encoded))
	assert.Contains(t, decoded, "### Minor Enhancements")
}

func TestAddMergeReference(t *testing.T) {
	historyFile := addMergeReference("", "Development Fixes", "Some great change", 1)
	assert.Equal(t, "## HEAD\n\n### Development Fixes\n\n  * Some great change (#1)\n", historyFile)

	historyFile = addMergeReference(
		"## HEAD",
		"Development Fixes", "Another great change!!!!!!!", 1)
	assert.Equal(t, "## HEAD\n\n### Development Fixes\n\n  * Another great change!!!!!!! (#1)\n", historyFile)

	historyFile = addMergeReference(
		"## HEAD\n\n### Development Fixes\n\n  * Some great change (#1)\n",
		"Development Fixes", "Another great change!!!!!!!", 1)
	assert.Equal(t, "## HEAD\n\n### Development Fixes\n\n  * Some great change (#1)\n  * Another great change!!!!!!! (#1)\n", historyFile)

	historyFile = addMergeReference(
		"## HEAD\n\n### Development Fixes\n\n  * Some great change (#1)\n",
		"Development Fixes", "Another great change for <science>!!!!!!!", 1)
	assert.Equal(t, "## HEAD\n\n### Development Fixes\n\n  * Some great change (#1)\n  * Another great change for &lt;science&gt;!!!!!!! (#1)\n", historyFile)

	jekyllHistory, err := ioutil.ReadFile("History.markdown")
	assert.NoError(t, err)
	historyFile = addMergeReference(string(jekyllHistory), "Development Fixes", "A marvelous change.", 41526)
	assert.Contains(t, historyFile, "* A marvelous change. (#41526)\n\n### Site Enhancements")
}
