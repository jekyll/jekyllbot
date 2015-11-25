package autopull

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/parkr/auto-reply/Godeps/_workspace/src/github.com/google/go-github/github"
	"github.com/parkr/auto-reply/common"
)

var (
	baseBranch *string = github.String("master")
)

type AutoPullHandler struct {
	client *github.Client
	repos  map[string]bool
}

// branchFromRef takes "refs/heads/pull/my-pull" and returns "pull/my-pull"
func branchFromRef(ref string) string {
	return strings.Replace(ref, "refs/heads/", "", 1)
}

func prBodyForPush(push github.PushEvent) string {
	return fmt.Sprintf(
		"PR automatically created for @%s.\n\n%s",
		*push.Commits[0].Author.Name,
		*push.Commits[0].Message,
	)
}

func newPRForPush(push github.PushEvent) *github.NewPullRequest {
	return &github.NewPullRequest{
		Title: push.Commits[0].Message,
		Head:  github.String(branchFromRef(*push.Ref)),
		Base:  github.String("master"),
		Body:  github.String(prBodyForPush(push)),
	}
}

func (h *AutoPullHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var push github.PushEvent
	err := json.NewDecoder(r.Body).Decode(&push)
	if err != nil {
		log.Println("error unmarshalling issue stuffs:", err)
		http.Error(w, "bad json", 400)
		return
	}

	if _, ok := h.repos[*push.Repo.FullName]; ok && strings.HasPrefix(*push.Ref, "pull/") {
		pr := newPRForPush(push)
		pull, _, err := h.client.PullRequests.Create(*push.Repo.Owner.Login, *push.Repo.Name, pr)
		if err != nil {
			log.Println("error unmarshalling issue stuffs:", err)
			http.Error(w, "bad json", 400)
			return
		}

		http.Error(w, "pr created: "+*pull.HTMLURL, 201)
	} else {
		log.Println("ignoring - ref doesn't match pull/* or not supported repo.")
		http.Error(w, "ignoring", 200)
	}
}

func NewHandler(client *github.Client, handledRepos []string) *AutoPullHandler {
	return &AutoPullHandler{
		client: client,
		repos:  common.SliceLookup(handledRepos),
	}
}
