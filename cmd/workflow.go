package cmd

import (
	"fmt"
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations/atlassian"
	"github.com/benchlabs/bub/integrations/github"
	"github.com/benchlabs/bub/utils"
	"log"
)

type Workflow struct {
	cfg      *core.Configuration
	git      *core.Git
	github   *github.GitHub
	jira     *atlassian.JIRA
	manifest *core.Manifest
}

func MustInitWorkflow(cfg *core.Configuration, manifest *core.Manifest) *Workflow {
	return &Workflow{
		cfg:      cfg,
		git:      core.InitGit(),
		github:   github.MustInitGitHub(cfg),
		jira:     atlassian.MustInitJIRA(cfg),
		manifest: manifest,
	}
}

func (wf *Workflow) Git() *core.Git {
	if wf.git == nil {
		wf.git = core.InitGit()
	}
	return wf.git
}

func (wf *Workflow) GitHub() *github.GitHub {
	if wf.github == nil {
		wf.github = github.MustInitGitHub(wf.cfg)
	}
	return wf.github
}

func (wf *Workflow) JIRA() *atlassian.JIRA {
	if wf.jira == nil {
		wf.jira = atlassian.MustInitJIRA(wf.cfg)
	}
	return wf.jira
}

func (wf *Workflow) MassUpdate(unstash bool) error {
	return core.ForEachRepo(func(repoDir string) (string, error) {
		return core.MustInitGit(repoDir).Sync(unstash)
	})
}

func (wf *Workflow) MassStart(unstash bool) error {
	issue, err := wf.JIRA().PickAssignedIssue()
	if err != nil {
		return err
	}

	return core.ForEachRepo(func(repo string) (string, error) {
		g := core.MustInitGit(repo)
		output, err := g.Sync(unstash)
		if err != nil {
			return output, err
		}
		return "", wf.JIRA().CreateBranchFromIssue(issue, repo, true)
	})
}

func (wf *Workflow) MassDiff() error {
	return core.ForEachRepo(func(repo string) (string, error) {
		g := core.MustInitGit(repo)
		return g.Diff()
	})
}

func (wf *Workflow) MassDone(noOperation bool) error {
	return core.ForEachRepo(func(repoDir string) (string, error) {
		g := core.MustInitGit(repoDir)
		if g.ContainedUncommittedChanges() {
			utils.ConditionalOp(fmt.Sprintf("%v - Committing.", repoDir), noOperation, func() error {
				return g.CommitWithBranchName()
			})
		}

		if !g.IsDifferentFromMainBranch() {
			log.Printf("%v - No commits. Skipping.", repoDir)
			return "", nil
		}

		utils.ConditionalOp(fmt.Sprintf("%v - Pushing", repoDir), noOperation, func() error {
			err := g.Push(wf.cfg)
			if err != nil {
				return err
			}
			return wf.GitHub().CreatePR("", "", repoDir)
		})
		return "", nil
	})
}

func (wf *Workflow) CreatePR(title, body string, review bool) error {
	if review || utils.AskForConfirmation("Transition issue?") {
		err := wf.JIRA().TransitionIssue("", "review")
		if err != nil {
			return err
		}
	}
	return wf.GitHub().CreatePR(title, body, "")
}

func (wf *Workflow) Log() error {
	c, err := wf.Git().PickCommit(wf.Git().Log())
	if err != nil {
		return err
	}
	return wf.OpenCommit(c)
}

func (wf *Workflow) OpenCommit(c *core.GitCommit) error {
	issueKey := wf.Git().GetIssueRegex().FindString(c.Subject)
	pr := wf.Git().GetPRRegex().FindStringSubmatch(c.Subject)

	openList := map[string]func() error{
		"GitHub Commit": func() error {
			return wf.GitHub().OpenCommit(wf.manifest, c)
		},
		"GitHub Compare with Master": func() error {
			return wf.GitHub().OpenCompareCommitsPage(wf.manifest, c, "master")
		},
	}
	if len(pr) > 2 && pr[2] != "" {
		openList["GitHub PR"] = func() error {
			return wf.GitHub().OpenPR(wf.manifest, pr[2])
		}
	}
	if issueKey != "" {
		openList["JIRA"] = func() error {
			return wf.JIRA().OpenIssueFromKey(issueKey, false)
		}
	}
	if len(openList) > 0 {
		var titles []string
		for k := range openList {
			titles = append(titles, k)
		}
		result, err := utils.PickItem("Open", titles)
		if err != nil {
			return err
		}
		return openList[result]()
	}
	fmt.Printf("%v", c)
	return nil
}
