package dependencies

import (
	"github.com/hashicorp/go-version"
	"github.com/jekyll/jekyllbot/ctx"
)

type Dependency interface {
	GetName() string                    // pre-populated upon creation
	GetConstraint() version.Constraints // pre-populated upon creation
	GetLatestVersion(context *ctx.Context) *version.Version
	IsOutdated(context *ctx.Context) bool
}
