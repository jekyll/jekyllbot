package jekyll

import (
	"strings"

	"github.com/pkg/errors"
)

type JekyllRepository struct {
	name string
}

// Always the Jekyll org.
func (r JekyllRepository) Owner() string {
	return "jekyll"
}

func (r JekyllRepository) Name() string {
	return r.name
}

// String returns NWO.
func (r JekyllRepository) String() string {
	return r.Owner() + "/" + r.Name()
}

func ParseRepository(repoNWO string) (Repository, error) {
	pieces := strings.Split(repoNWO, "/")
	if len(pieces) != 2 {
		return nil, errors.Errorf("invalid repo NWO: %q", repoNWO)
	}
	return NewRepository(pieces[0], pieces[1]), nil
}

func NewRepository(owner, repo string) Repository {
	return GitHubRepository{owner, repo}
}

type GitHubRepository struct {
	owner string
	name  string
}

// Always the Jekyll org.
func (r GitHubRepository) Owner() string {
	return r.owner
}

func (r GitHubRepository) Name() string {
	return r.name
}

// String returns NWO.
func (r GitHubRepository) String() string {
	return r.Owner() + "/" + r.Name()
}

type Repository interface {
	Owner() string
	Name() string
	String() string
}

var DefaultRepos = []Repository{
	JekyllRepository{name: "github-metadata"},
	JekyllRepository{name: "jekyll"},
	JekyllRepository{name: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-compose"},
	JekyllRepository{name: "jekyll-feed"},
	JekyllRepository{name: "jekyll-gist"},
	JekyllRepository{name: "jekyll-import"},
	JekyllRepository{name: "jekyll-redirect-from"},
	JekyllRepository{name: "jekyll-sass-converter"},
	JekyllRepository{name: "jekyll-seo-tag"},
	JekyllRepository{name: "jekyll-sitemap"},
	JekyllRepository{name: "jekyll-watch"},
	JekyllRepository{name: "jemoji"},
	JekyllRepository{name: "minima"},
	JekyllRepository{name: "directory"}, // formerly plugins
}
