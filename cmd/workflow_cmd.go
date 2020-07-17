package cmd

import (
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations/atlassian"
	"github.com/benchlabs/bub/integrations/github"
	"github.com/benchlabs/bub/utils"
	"github.com/urfave/cli"
	"os"
)

func buildWorkflowCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	transition := "t"
	noOperation := "noop"
	compare := "compare-only"
	unstash := "unstash"
	unstashDesc := "Unstash changes at the end of the update."
	return []cli.Command{
		buildJIRAOpenBoardCmd(cfg),
		buildJIRAClaimIssueCmd(cfg),
		buildJIRAOpenIssueCmd(cfg),
		buildJIRAViewIssueCmd(cfg),
		buildJIRACommentOnIssuesCmd(cfg),
		buildJIRAListAssignedIssuesCmd(cfg),
		{
			Name:    "new-branch",
			Aliases: []string{"n", "new"},
			Usage:   "Checkout a new branch based on JIRA issues assigned to you.",
			Action: func(c *cli.Context) error {
				return atlassian.MustInitJIRA(cfg).CreateBranchFromAssignedIssue()
			},
		},
		{
			Name:    "checkout-branch",
			Aliases: []string{"b"},
			Usage:   "Checkout an existing branch.",
			Action: func(c *cli.Context) error {
				return core.InitGit().CheckoutBranch()
			},
		},
		{
			Name:    "commit",
			Aliases: []string{"c"},
			Usage:   "MESSAGE [OPTS]...",
			Action: func(c *cli.Context) error {
				message := ""
				if len(c.Args()) > 0 {
					message = c.Args().Get(0)
				}
				return core.InitGit().CommitWithIssueKey(cfg, message, c.Args().Tail())
			},
		},
		{
			Name:    "pull-request",
			Aliases: []string{"pr"},
			Usage:   "Creates a PR for the current branch.",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: compare, Usage: "Open only the compare page (PR creation page)."},
				cli.BoolFlag{Name: transition, Usage: "Transition the issue to review."},
			},
			Action: func(c *cli.Context) error {
				if c.Bool(compare) {
					err := core.MustInitGit("").Push(cfg)
					if err != nil {
						return err
					}
					return github.MustInitGitHub(cfg).OpenCompareBranchPage(manifest)
				}
				var title, body string
				if len(c.Args()) > 0 {
					title = c.Args().Get(0)
				}
				if len(c.Args()) > 1 {
					body = c.Args().Get(1)
				}
				return MustInitWorkflow(cfg, manifest).CreatePR(title, body, c.Bool("transition"))
			},
		},
		buildJIRATransitionIssueCmd(cfg),
		{
			Name:    "log",
			Aliases: []string{"l"},
			Usage:   "Show git log and open PR, JIRA ticket, etc.",
			Action: func(c *cli.Context) error {
				return MustInitWorkflow(cfg, manifest).Log()
			},
		},
		{
			Name:    "mass",
			Aliases: []string{"m"},
			Usage:   "Mass repo changes. EXPERIMENTAL",
			Subcommands: []cli.Command{
				{
					Name:  "start",
					Usage: "Clean the repository, checkout main branch, pull and create new branch.",
					Flags: []cli.Flag{
						cli.BoolFlag{Name: unstash, Usage: unstashDesc},
					},
					Action: func(c *cli.Context) error {
						if !utils.AskForConfirmation("You will lose existing changes.") {
							os.Exit(1)
						}
						return MustInitWorkflow(cfg, manifest).MassStart(c.Bool(unstash))
					},
				},
				{
					Name:    "diff",
					Aliases: []string{"d"},
					Usage:   "Shows the diff of all repos.",
					Action: func(c *cli.Context) error {
						return MustInitWorkflow(cfg, manifest).MassDiff()
					},
				},
				{
					Name:  "done",
					Usage: "Commit changes and create PRs. To be used after running '... start' and you made your changes.",
					Flags: []cli.Flag{
						cli.BoolFlag{Name: noOperation, Usage: "Do not do any actions."},
					},
					Action: func(c *cli.Context) error {
						if !utils.AskForConfirmation("You will create a PR for every changes made to the repo. Use `--noop` to check first. Continue?") {
							os.Exit(1)
						}
						return MustInitWorkflow(cfg, manifest).MassDone(c.Bool(noOperation))
					},
				},
				{
					Name:    "update",
					Aliases: []string{"u"},
					Usage:   "Clean the repository, checkout main branch and pull.",
					Flags: []cli.Flag{
						cli.BoolFlag{Name: unstash, Usage: unstashDesc},
					},
					Action: func(c *cli.Context) error {
						if !utils.AskForConfirmation("You will lose existing changes.") {
							os.Exit(1)
						}
						return MustInitWorkflow(cfg, manifest).MassUpdate(c.Bool(unstash))
					},
				},
			},
		},
	}
}
