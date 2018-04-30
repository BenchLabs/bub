package main

import (
	"github.com/benchlabs/bub/cmd"
	"github.com/urfave/cli"
	"os"
)

func main() {
	app := cli.NewApp()
	app.Name = "bub"
	app.Usage = "A tool for all your needs."
	app.Version = "0.59.0"
	app.EnableBashCompletion = true
	app.Commands = cmd.BuildCmds()
	app.Run(os.Args)
}
