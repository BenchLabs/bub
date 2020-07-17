package cmd

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations"
	"github.com/benchlabs/bub/integrations/atlassian"
	"github.com/benchlabs/bub/integrations/aws"
	"github.com/benchlabs/bub/integrations/ci"
	"github.com/benchlabs/bub/integrations/github"
	"github.com/benchlabs/bub/integrations/vault"
	"github.com/benchlabs/bub/utils"
	"github.com/urfave/cli"
)

func buildSetupCmd() cli.Command {
	resetCredentials := "reset-credentials"
	return cli.Command{
		Name:  "setup",
		Usage: "Setup bub on your machine.",
		Flags: []cli.Flag{
			cli.BoolFlag{Name: resetCredentials, Usage: "Prompt you to re-enter credentials."},
		},
		Action: func(c *cli.Context) error {
			core.MustSetupConfig()
			// Reloading the config
			cfg, _ := core.LoadConfiguration()
			cfg.ResetCredentials = c.Bool(resetCredentials)
			aws.MustSetupConfig()
			atlassian.MustSetupJIRA(cfg)
			atlassian.MustSetupConfluence(cfg)
			github.MustSetupGitHub(cfg)
			ci.MustSetupJenkins(cfg)
			vault.MustSetupVault(cfg)
			log.Println("Done.")
			return nil
		},
	}
}

func buildCircleCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	return []cli.Command{
		{
			Name:    "trigger",
			Usage:   "Trigger the current branch of the current repo and wait for success.",
			Aliases: []string{"t"},
			Action: func(c *cli.Context) error {
				return ci.MustInitCircle(cfg).TriggerAndWaitForSuccess(manifest)
			},
		},
		{
			Name:    "check",
			Usage:   "Check the build status of the current commit.",
			Aliases: []string{"c"},
			Action: func(c *cli.Context) error {
				return ci.MustInitCircle(cfg).CheckBuildStatus(manifest)
			},
		},
		{
			Name:    "artifact",
			Usage:   "Get the build artifact of the current commit. You need to provide the artifact file name as an argument.",
			Aliases: []string{"a"},
			Action: func(c *cli.Context) error {
				var dir string

				if c.NArg() < 1 {
					return fmt.Errorf("You have too few arguments. Please provide the name of the artifact you want to download.")
				}
				if c.NArg() > 1 {
					return fmt.Errorf("You have too many arguments. The only argument accepted is the name of the artifact you want to download.")
				}

				fname := c.Args().Get(0)
				dir = c.String("path")
				if len(dir) == 0 {
					d, err := os.Getwd()
					if err != nil {
						return err
					}
					dir = d
				}

				_, err := os.Stat(dir)
				if err != nil {
					return err
				}

				return ci.MustInitCircle(cfg).DownloadArtifact(manifest, fname, path.Join(dir, fname))
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "path, p",
					Usage: "path to download the artifact to",
				},
			},
		},
		{
			Name:    "open",
			Usage:   "Open Circle for the current repository.",
			Aliases: []string{"t"},
			Action: func(c *cli.Context) error {
				return ci.OpenCircle(cfg, manifest, false)
			},
		},
		{
			Name:    "circle",
			Usage:   "Opens the result for the current branch.",
			Aliases: []string{"b"},
			Action: func(c *cli.Context) error {
				return ci.OpenCircle(cfg, manifest, true)
			},
		},
	}
}

func buildSplunkCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	return []cli.Command{
		{
			Name:    "production",
			Aliases: []string{"p"},
			Usage:   "Open the service production logs.",
			Action: func(c *cli.Context) error {
				return integrations.OpenSplunk(cfg, manifest, false)
			},
		},
		{
			Name:    "staging",
			Aliases: []string{"s"},
			Usage:   "Open the service staging logs.",
			Action: func(c *cli.Context) error {
				return integrations.OpenSplunk(cfg, manifest, true)
			},
		},
	}
}

func buildRepositoryCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	slackFormat := "slack-format"
	noSlackAt := "slack-no-at"
	noFetch := "no-fetch"
	return []cli.Command{
		{
			Name:  "synchronize",
			Usage: "Synchronize the all the active repositories.",
			Action: func(c *cli.Context) error {
				message := `

STOP!

This command will clone and/or Update all 'active' Bench repositories.
Existing work will be stashed and pull the main branch. Your work won't be lost, but be careful.
Please make sure you are in the directory where you store your repos and not a specific repo.

Continue?`
				if !c.Bool("force") && !utils.AskForConfirmation(message) {
					os.Exit(1)
				}
				return core.SyncRepositories()
			},
		},
		{
			Name:    "pending",
			Aliases: []string{"p"},
			Usage:   "List diff between the previous version and the next one.",
			Flags: []cli.Flag{
				cli.BoolFlag{Name: slackFormat, Usage: "Format the result for slack."},
				cli.BoolFlag{Name: noSlackAt, Usage: "Do not add @person at the end."},
				cli.BoolFlag{Name: noFetch, Usage: "Do not fetch tags."},
			},
			Action: func(c *cli.Context) error {
				if !c.Bool(noFetch) {
					err := core.InitGit().FetchTags()
					if err != nil {
						return err
					}
				}
				previousVersion := "production"
				if len(c.Args()) > 0 {
					previousVersion = c.Args().Get(0)
				}
				nextVersion := "HEAD"
				if len(c.Args()) > 1 {
					nextVersion = c.Args().Get(1)
				}
				core.InitGit().PendingChanges(cfg, manifest, previousVersion, nextVersion, c.Bool(slackFormat), c.Bool(noSlackAt))
				return nil
			},
		},
	}
}
