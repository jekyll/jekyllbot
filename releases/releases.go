package releases

import (
	"fmt"
	"sort"

	"github.com/google/go-github/v53/github"
	"github.com/hashicorp/go-version"
	"github.com/jekyll/jekyllbot/ctx"
	"github.com/jekyll/jekyllbot/jekyll"
)

func LatestRelease(context *ctx.Context, repo jekyll.Repository) (*github.RepositoryRelease, error) {
	releases, _, err := context.GitHub.Repositories.ListReleases(context.Context(), repo.Owner(), repo.Name(), &github.ListOptions{PerPage: 300})
	if err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, nil
	}

	versionMap := map[string]*github.RepositoryRelease{}
	versions := []*version.Version{}
	for _, release := range releases {
		v, err := version.NewVersion(release.GetTagName())
		if err != nil {
			continue
		}
		versionMap[v.String()] = release
		versions = append(versions, v)
	}

	// After this, the versions are properly sorted
	sort.Sort(sort.Reverse(version.Collection(versions)))

	if latestRelease, ok := versionMap[versions[0].String()]; ok {
		return latestRelease, nil
	}

	return nil, fmt.Errorf("%s: couldn't find %s in versions %+v", repo, versions[0], versions)
}

func CommitsSinceRelease(context *ctx.Context, repo jekyll.Repository, latestRelease *github.RepositoryRelease) (int, error) {
	defaultBranch := "master" // fallback
	repoInfo, _, err := context.GitHub.Repositories.Get(context.Context(), repo.Owner(), repo.Name())
	if err != nil {
		fmt.Printf("releases: error getting default branch for %s/%s", repo.Owner(), repo.Name())
	}
	if repoInfo.GetDefaultBranch() != "" {
		defaultBranch = repoInfo.GetDefaultBranch()
	}
	comparison, _, err := context.GitHub.Repositories.CompareCommits(
		context.Context(),
		repo.Owner(), repo.Name(),
		latestRelease.GetTagName(), defaultBranch,
		&github.ListOptions{PerPage: 1000},
	)
	if err != nil {
		return -1, fmt.Errorf("error fetching commit comparison for %s...%s for %s: %v", latestRelease.GetTagName(), defaultBranch, repo, err)
	}

	return comparison.GetTotalCommits(), nil
}
