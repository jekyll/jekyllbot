package jekyll

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

const jekyllStr = "jekyll"

type JekyllRepository struct {
	name        string
	gemspecName string
}

// Always the Jekyll org.
func (r JekyllRepository) Owner() string {
	return jekyllStr
}

func (r JekyllRepository) Name() string {
	return r.name
}

// String returns NWO.
func (r JekyllRepository) String() string {
	return r.Owner() + "/" + r.Name()
}

func (r JekyllRepository) GemspecName() string {
	return r.gemspecName
}

func ParseRepository(repoNWO string) (Repository, error) {
	pieces := strings.Split(repoNWO, "/")
	if len(pieces) != 2 {
		return nil, errors.Errorf("invalid repo NWO: %q", repoNWO)
	}
	return NewRepository(pieces[0], pieces[1]), nil
}

func ParseRepositoryFromURL(urlStr string) (Repository, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	pieces := strings.Split(u.Path, "/")
	if len(pieces) < 2 {
		return nil, errors.Errorf("url has no repo: %q", urlStr)
	}
	return NewRepository(pieces[1], pieces[2]), nil
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

func (r GitHubRepository) GemspecName() string {
	return "GitHubRepository.GemspecName is not implemented yet"
}

type Repository interface {
	Owner() string
	Name() string
	String() string
	GemspecName() string
}

var DefaultRepos = []Repository{
	JekyllRepository{name: "github-metadata", gemspecName: "jekyll-github-metadata"},
	JekyllRepository{name: "jekyll", gemspecName: "jekyll"},
	JekyllRepository{name: "jekyll-coffeescript", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-compose", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-feed", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-gist", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-import", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-redirect-from", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-sass-converter", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-seo-tag", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-sitemap", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jekyll-watch", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "jemoji", gemspecName: "jekyll-coffeescript"},
	JekyllRepository{name: "minima", gemspecName: "minima"},
	JekyllRepository{name: "directory"}, // formerly plugins
}
