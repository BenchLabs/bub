package cmd

import (
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/integrations/ci"
	"github.com/urfave/cli"
)

func buildJenkinsCmds(cfg *core.Configuration, manifest *core.Manifest) []cli.Command {
	return []cli.Command{
		{
			Name:    "master",
			Aliases: []string{"m"},
			Usage:   "Opens the (web) main build.",
			Action: func(c *cli.Context) error {
				return ci.MustInitJenkins(cfg, manifest).OpenPage()
			},
		},
		{
			Name:    "console",
			Aliases: []string{"c"},
			Usage:   "Opens the (web) console of the last build of main branch.",
			Action: func(c *cli.Context) error {
				return ci.MustInitJenkins(cfg, manifest).OpenPage("lastBuild/consoleFull")
			},
		},
		{
			Name:    "jobs",
			Aliases: []string{"j"},
			Usage:   "Shows the console output of the last build.",
			Action: func(c *cli.Context) error {
				ci.MustInitJenkins(cfg, manifest).ShowConsoleOutput()
				return nil
			},
		},
		{
			Name:    "artifacts",
			Aliases: []string{"a"},
			Usage:   "Get the previous build's artifacts.",
			Action: func(c *cli.Context) error {
				return ci.MustInitJenkins(cfg, manifest).GetArtifacts()
			},
		},
		{
			Name:    "build",
			Aliases: []string{"b"},
			Flags: []cli.Flag{
				cli.BoolFlag{Name: "no-wait", Usage: "Do not wait for the job to be completed."},
				cli.BoolFlag{Name: "force", Usage: "Trigger job regardless if a build running."},
			},
			Usage: "Trigger build of the current branch.",
			Action: func(c *cli.Context) error {
				ci.MustInitJenkins(cfg, manifest).BuildJob(c.Bool("no-wait"), c.Bool("force"))
				return nil
			},
		},
	}
}
