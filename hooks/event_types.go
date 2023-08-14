package hooks

type EventType string

var (
	CheckRunEvent                 EventType = "check_run"
	CheckSuiteEvent               EventType = "check_suite"
	CodeScanningAlertEvent        EventType = "code_scanning_alert"
	CommitCommentEvent            EventType = "commit_comment"
	CreateEvent                   EventType = "create"
	DeleteEvent                   EventType = "delete"
	DeploymentEvent               EventType = "deployment"
	DeploymentStatusEvent         EventType = "deployment_status"
	ForkEvent                     EventType = "fork"
	GollumEvent                   EventType = "gollum"
	IssueCommentEvent             EventType = "issue_comment"
	IssuesEvent                   EventType = "issues"
	MemberEvent                   EventType = "member"
	MembershipEvent               EventType = "membership"
	PageBuildEvent                EventType = "page_build"
	PublicEvent                   EventType = "public"
	PullRequestEvent              EventType = "pull_request"
	PullRequestReviewEvent        EventType = "pull_request_review"
	PullRequestReviewCommentEvent EventType = "pull_request_review_comment"
	PushEvent                     EventType = "push"
	ReleaseEvent                  EventType = "release"
	RepositoryEvent               EventType = "repository"
	StatusEvent                   EventType = "status"
	TeamAddEvent                  EventType = "team_add"
	WatchEvent                    EventType = "watch"
	WorkflowJobEvent              EventType = "workflow_job"
	WorkflowRunEvent              EventType = "workflow_run"

	pingEvent EventType = "ping"
)

func (e EventType) String() string {
	return string(e)
}

type pingEventPayload struct {
	Zen string `json:"zen"`
}
