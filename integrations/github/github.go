package github

import (
	"context"
	"fmt"
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/utils"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

type GitHub struct {
	cfg    *core.Configuration
	client *github.Client
}

func MustInitGitHub(cfg *core.Configuration) *GitHub {
	ctx := context.Background()
	mustLoadGitHubToken(cfg)
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GitHub.Token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	return &GitHub{cfg, client}
}

func mustLoadGitHubToken(cfg *core.Configuration) {
	err := core.LoadKeyringItem("GitHub User", &cfg.GitHub.Username)
	if err != nil {
		log.Fatalf("Failed to set GitHub User: %v", err)
	}
	err = core.LoadKeyringItem("GitHub Token", &cfg.GitHub.Token)
	if err != nil {
		log.Fatalf("Failed to set GitHub Token: %v", err)
	}
}

func MustSetupGitHub(cfg *core.Configuration) {
	if utils.AskForConfirmation(
		"Create a new GitHub Token. " +
			"Grant 'Full control of private repositories'.\n" +
			"Open the GitHub new token page?") {
		utils.OpenURI("https://github.com/settings/tokens/new")
	}
	mustLoadGitHubToken(cfg)
}

func (gh *GitHub) CreatePR(title, body, repoDir string) error {
	g := core.MustInitGit(repoDir)
	err := g.Push(gh.cfg)
	if err != nil {
		return err
	}
	err = g.Fetch()
	if err != nil {
		return err
	}
	branch := g.GetCurrentBranch()
	base := "master"
	if title == "" {
		subjects := g.LogNotInMainSubjects()
		if len(subjects) == 1 {
			title = subjects[0]
		} else {
			title = g.GetTitleFromBranchName()
		}
	}

	if body == "" {
		body = g.LogNotInMainBody()
	}

	root, err := g.GetRepositoryRootPath()
	if err != nil {
		return err
	}
	prTemplateFile := path.Join(root, ".github", "PULL_REQUEST_TEMPLATE.md")
	exists, err := utils.PathExists(prTemplateFile)
	if err != nil {
		return err
	}

	if exists {
		content, err := ioutil.ReadFile(prTemplateFile)
		if err != nil {
			return err
		}
		body = body + "\n\n" + string(content)
	}
	ctx := context.Background()
	org := gh.cfg.GitHub.Organization
	repo := g.GetCurrentRepositoryName()

	request := github.NewPullRequest{Head: &branch, Base: &base, Title: &title, Body: &body}
	pr, _, err := gh.client.PullRequests.Create(ctx, org, repo, &request)

	if err != nil {
		prListOptions := github.PullRequestListOptions{Head: branch, Base: base}
		existingPRs, _, err := gh.client.PullRequests.List(ctx, org, repo, &prListOptions)
		if len(existingPRs) > 0 {
			log.Print("Existing PR found.")
			return utils.OpenURI(*existingPRs[0].HTMLURL)
		}
		return err
	}

	reviewers, err := gh.ListReviewers()
	if err != nil {
		return err
	}
	if len(reviewers) > 0 {
		reviewersRequest := github.ReviewersRequest{Reviewers: reviewers}
		pr, _, err = gh.client.PullRequests.RequestReviewers(ctx, org, repo, *pr.Number, reviewersRequest)

		if err != nil {
			return err
		}

	}
	return utils.OpenURI(*pr.HTMLURL)
}

func (gh *GitHub) OpenPage(m *core.Manifest, p ...string) error {
	base := []string{
		"https://github.com",
		gh.cfg.GitHub.Organization,
		m.Repository,
	}
	return utils.OpenURI(append(base, p...)...)
}

func (gh *GitHub) OpenPR(m *core.Manifest, pr string) error {
	return gh.OpenPage(m, "pull", pr, "files")
}

func (gh *GitHub) OpenCommit(m *core.Manifest, commit *core.GitCommit) error {
	return gh.OpenPage(m, "commit", commit.Hash)
}

func (gh *GitHub) OpenCompareCommitsPage(m *core.Manifest, commit *core.GitCommit, ref string) error {
	return gh.OpenPage(m, "compare", commit.Hash+"..."+ref)
}

func (gh *GitHub) OpenCompareBranchPage(m *core.Manifest) error {
	return gh.OpenPage(m, "compare", "master..."+m.Branch)
}

func (gh *GitHub) ListBranches(maxAge int) error {
	type branch struct {
		Repository, Branch, Name, Email, PRURL string
		Age                                    int
	}
	ctx := context.Background()
	authors := map[string][]branch{}

	orgOptions := github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 250}}
	org := gh.cfg.GitHub.Organization
	repos, _, err := gh.client.Repositories.ListByOrg(ctx, org, &orgOptions)
	if err != nil {
		return err
	}
	for _, r := range repos {
		log.Print(*r.Name)
		if *r.Fork {
			continue
		}
		branches, _, err := gh.client.Repositories.ListBranches(ctx, org, *r.Name, &github.ListOptions{PerPage: 250})
		if err != nil {
			return err
		}
		prs, _, err := gh.client.PullRequests.List(ctx, org, *r.Name, &github.PullRequestListOptions{State: "open"})
		if err != nil {
			return err
		}
		pullRequests := map[string]*github.PullRequest{}
		for _, pr := range prs {
			pullRequests[*pr.Head.SHA] = pr
		}
		for _, b := range branches {
			if *b.Name == "master" {
				continue
			}
			b, _, err := gh.client.Repositories.GetBranch(ctx, org, *r.Name, url.PathEscape(*b.Name))
			if err != nil {
				return err
			}
			author := b.Commit.Commit.GetAuthor()
			age := int(time.Since(*author.Date).Hours() / 24)
			if age > maxAge {
				sha := *b.Commit.SHA
				pr := pullRequests[sha]
				prURL := ""
				if pr != nil {
					prURL = *pr.HTMLURL
				}
				br := branch{Repository: *r.Name, Branch: *b.Name, Name: *author.Name, Email: *author.Email, Age: int(age), PRURL: prURL}
				authors[br.Name] = append(authors[br.Name], br)
			}
		}
	}
	for author, branches := range authors {
		fmt.Println("\n" + author)
		for _, b := range branches {
			fmt.Printf("https://github.com/%v/%v/branches/yours %v %v\n", org, b.Repository, b.Branch, b.PRURL)
		}
	}
	return nil
}

func (gh *GitHub) SearchIssues(issueType, role string, closed, openAll bool) error {
	ctx := context.Background()
	if issueType == "" {
		issueType = "pr"
	}
	if role == "" {
		role = "author"
	}
	state := "open"
	if closed {
		state = "closed"
	}
	prs, _, err := gh.client.Search.Issues(ctx, fmt.Sprintf("type:%v state:%v %v:%v", issueType, state, role, gh.cfg.GitHub.Username), &github.SearchOptions{Sort: "author-date"})
	if err != nil {
		return err
	}
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "#\tTitle\tURL")
	for _, pr := range prs.Issues {
		fmt.Fprintln(table, strings.Join([]string{strconv.Itoa(*pr.Number), *pr.Title, *pr.HTMLURL}, "\t"))
		if openAll {
			utils.OpenURI(*pr.HTMLURL)
		}
	}
	table.Flush()
	return nil
}
