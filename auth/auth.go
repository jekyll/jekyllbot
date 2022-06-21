// auth provides a means of determining use permissions on GitHub.com for repositories.
package auth

import (
	"fmt"
	"log"

	"github.com/google/go-github/v45/github"
	"github.com/jekyll/jekyllbot/ctx"
)

type teamMembershipAnswer string

const (
	teamMembershipUnknown teamMembershipAnswer = ""
	teamMembershipYes     teamMembershipAnswer = "YES"
	teamMembershipNo      teamMembershipAnswer = "NO"
)

var (
	teamsCache             = map[string][]*github.Team{}
	teamHasPushAccessCache = map[string]*github.Repository{}
	teamMembershipCache    = map[string]teamMembershipAnswer{}
	orgOwnersCache         = map[string][]*github.User{}
)

type authenticator struct {
	context *ctx.Context
}

func CommenterHasPushAccess(context *ctx.Context, owner, repo, commenterLogin string) bool {
	auth := authenticator{context: context}
	orgTeams := auth.teamsForOrg(owner)
	for _, team := range orgTeams {
		if auth.isTeamMember(*team.Organization.ID, *team.ID, commenterLogin) &&
			auth.teamHasPushAccess(*team.Organization.ID, *team.ID, owner, repo) {
			return true
		}
	}
	return false
}

func UserIsOrgOwner(context *ctx.Context, org, login string) bool {
	auth := authenticator{context: context}
	for _, owner := range auth.ownersForOrg(org) {
		if *owner.Login == login {
			return true
		}
	}
	return false
}

func (auth authenticator) isTeamMember(orgID, teamID int64, login string) bool {
	cacheKey := auth.cacheKeyIsTeamMember(orgID, teamID, login)
	if _, ok := teamMembershipCache[cacheKey]; !ok {
		membership, resp, err := auth.context.GitHub.Teams.GetTeamMembershipByID(auth.context.Context(),
			orgID,
			teamID,
			login,
		)
		if resp.StatusCode == 404 {
			teamMembershipCache[cacheKey] = teamMembershipNo
			return false
		}
		if err != nil {
			log.Printf("ERROR performing GetTeamMembershipByID(%d, %d, \"%s\"): %v", orgID, teamID, login, err)
			return false
		}
		if membership.GetState() == "active" {
			teamMembershipCache[cacheKey] = teamMembershipYes
		} else {
			teamMembershipCache[cacheKey] = teamMembershipNo
		}
	}
	return teamMembershipCache[cacheKey] == teamMembershipYes
}

func (auth authenticator) teamHasPushAccess(orgID, teamID int64, owner, repo string) bool {
	cacheKey := auth.cacheKeyTeamHashPushAccess(orgID, teamID, owner, repo)
	if _, ok := teamHasPushAccessCache[cacheKey]; !ok {
		repository, _, err := auth.context.GitHub.Teams.IsTeamRepoByID(
			auth.context.Context(), orgID, teamID, owner, repo)
		if err != nil {
			log.Printf("ERROR performing IsTeamRepo(%d, \"%s\", \"%s\"): %v", teamID, owner, repo, err)
			return false
		}
		if repository == nil {
			return false
		}
		teamHasPushAccessCache[cacheKey] = repository
	}
	permissions := teamHasPushAccessCache[cacheKey].GetPermissions()
	return permissions["push"] || permissions["admin"]
}

func (auth authenticator) teamsForOrg(org string) []*github.Team {
	if _, ok := teamsCache[org]; !ok {
		teamz, _, err := auth.context.GitHub.Teams.ListTeams(
			auth.context.Context(),
			org,
			&github.ListOptions{Page: 0, PerPage: 100},
		)
		if err != nil {
			log.Printf("ERROR performing ListTeams(\"%s\"): %v", org, err)
			return nil
		}
		orgData, _, err := auth.context.GitHub.Organizations.Get(auth.context.Context(), org)
		if err != nil {
			log.Printf("ERROR performing GetOrg(\"%s\"): %v", org, err)
			return nil
		}
		for _, team := range teamz {
			team.Organization = orgData
		}
		teamsCache[org] = teamz
	}
	return teamsCache[org]
}

func (auth authenticator) ownersForOrg(org string) []*github.User {
	if _, ok := orgOwnersCache[org]; !ok {
		owners, _, err := auth.context.GitHub.Organizations.ListMembers(
			auth.context.Context(),
			org,
			&github.ListMembersOptions{Role: "admin"}, // owners
		)
		if err != nil {
			auth.context.Log("ERROR performing ListMembers(\"%s\"): %v", org, err)
			return nil
		}
		orgOwnersCache[org] = owners
	}
	return orgOwnersCache[org]
}

func (auth authenticator) cacheKeyIsTeamMember(orgID, teamID int64, login string) string {
	return fmt.Sprintf("%d_%d_%s", orgID, teamID, login)
}

func (auth authenticator) cacheKeyTeamHashPushAccess(orgID, teamID int64, owner, repo string) string {
	return fmt.Sprintf("%d_%d_%s_%s", orgID, teamID, owner, repo)
}
