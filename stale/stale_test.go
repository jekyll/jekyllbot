package stale

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v46/github"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/stretchr/testify/assert"
)

func TestIsUpdatedWithinDuration(t *testing.T) {
	twoMonthsAgo := time.Now().AddDate(0, -2, 0)
	dormantDuration := time.Since(twoMonthsAgo)
	config := Configuration{DormantDuration: dormantDuration}

	cases := []struct {
		updatedAtDate                      time.Time
		isUpdatedWithinDurationReturnValue bool
	}{
		{time.Now().AddDate(-1, 0, 0), false},
		{time.Now().AddDate(0, -2, -1), false},
		{time.Now().AddDate(0, -1, -30), true},
		{time.Now().AddDate(0, -1, -29), true},
		{time.Now(), true},
	}

	for _, testCase := range cases {
		issue := &github.Issue{UpdatedAt: &testCase.updatedAtDate}
		assert.Equal(t,
			testCase.isUpdatedWithinDurationReturnValue,
			isUpdatedWithinDuration(issue, config),
			fmt.Sprintf(
				"date='%s' config.DormantDuration='%s' time.Since(date)='%s'",
				testCase.updatedAtDate,
				config.DormantDuration,
				time.Since(testCase.updatedAtDate)),
		)
	}
}

func TestCloseIssue(t *testing.T) {
	setup()
	defer teardown()
	context := ctx.NewTestContext()
	context.SetRepo("fakeowner", "fakerepo")
	context.GitHub = client
	issue := &github.Issue{Number: github.Int(123)}
	githubAPIPath := fmt.Sprintf("/repos/%s/%s/issues/%d", context.Repo.Owner, context.Repo.Name, issue.GetNumber())

	request := &github.IssueRequest{}
	mux.HandleFunc(githubAPIPath, func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PATCH")
		if err := json.NewDecoder(r.Body).Decode(request); err != nil {
			t.Fatalf("expected no error decoding json request body, got: %+v", err)
		}
		if request.GetState() != "closed" {
			t.Fatalf("expected request state to be 'closed', got: %s", request.GetState())
		}
		if request.GetStateReason() != "not_planned" {
			t.Fatalf("expected request state_reason to be 'not_planned', got: %s", request.GetStateReason())
		}
		w.WriteHeader(http.StatusOK)
	})

	if err := closeIssue(context, issue); err != nil {
		t.Fatalf("expected no error closing issue, got: %+v", err)
	}
}
