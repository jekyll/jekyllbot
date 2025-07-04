package stale

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v73/github"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/stretchr/testify/assert"
)

func TestIsUpdatedWithinDuration(t *testing.T) {
	now := time.Now()
	twoMonthsAgo := now.AddDate(0, 0, -60)
	dormantDuration := time.Since(twoMonthsAgo)
	config := Configuration{DormantDuration: dormantDuration}

	cases := []struct {
		scenarioID                         int
		updatedAtDate                      time.Time
		isUpdatedWithinDurationReturnValue bool
	}{
		// One year back, not updated within 60 days
		{1, now.AddDate(-1, 0, 0), false},
		// 61 days back, not updated within 60 days
		{2, now.AddDate(0, 0, -61), false},
		// 59 days back, updated within 60 days
		{3, now.AddDate(0, 0, -59), true},
		// 58 days back, updated within 60 days
		{4, now.AddDate(0, 0, -58), true},
		// One month back, updated within 60 days
		{5, now, true},
	}

	for _, testCase := range cases {
		issue := &github.Issue{UpdatedAt: &github.Timestamp{Time: testCase.updatedAtDate}}
		assert.Equal(t,
			testCase.isUpdatedWithinDurationReturnValue,
			isUpdatedWithinDuration(issue, config),
			fmt.Sprintf(
				"scenario=%d\ndate=%q\ncutoff=%q\nconfig.DormantDuration=%q\ntime.Since(date)=%q",
				testCase.scenarioID,
				testCase.updatedAtDate,
				now.Add(-config.DormantDuration),
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
