package issuecomment

import (
	"github.com/google/go-github/v53/github"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/jekyll/jekyllbot/labeler"
)

func StaleUnlabeler(context *ctx.Context, event interface{}) error {
	comment, ok := event.(*github.IssueCommentEvent)
	if !ok {
		return context.NewError("StaleUnlabeler: not an issue comment event")
	}

	if *comment.Action != "created" {
		return nil
	}

	if context.GitHubAuthedAs(*comment.Sender.Login) {
		return nil // heh.
	}

	owner, name, number := *comment.Repo.Owner.Login, *comment.Repo.Name, *comment.Issue.Number
	err := labeler.RemoveLabelIfExists(context, owner, name, number, "stale")
	if err != nil {
		return context.NewError("StaleUnlabeler: error removing label on %s/%s#%d: %v", owner, name, number, err)
	}

	return nil
}
