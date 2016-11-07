package main

import (
	"fmt"
	"github.com/docopt/docopt-go"
	"gopkg.in/yaml.v2"
	"log"
	"os"
)

func main() {
	version := "bub 0.4.2-experiment"
	usage := `bub.

Usage:
  bub setup
  bub list [--full]
  bub repository sync [--force]
  bub manifest update [--artifact-version <value>]
  bub manifest validate
  bub eb
  bub eb events
  bub ec2 [INSTANCE_NAME] [COMMAND ...]
  bub gh repo
  bub gh issues
  bub gh pr
  bub gh branches
  bub gh compare
  bub gh raml
  bub jenkins
  bub jenkins console
  bub jenkins trigger
  bub splunk
  bub splunk staging
  bub docs
  bub circle
  bub circle branch
  bub -h | --help
  bub --version

Arguments:
  INSTANCE_NAME                Optional EC2 instance name.
  COMMAND                      Optional command to run on the EC2 instance. e.g.: jstack

Options:
  -h --help                    Show this screen.
  --full                       List every defails contained in every manifests.
  --artifact-version <value>   Artifact version [default: n/a].
  --force                      Force sync, wihtout prompt.
  --version                    Version of the service to update.`

	args, _ := docopt.Parse(usage, nil, true, version, false)

	if args["list"].(bool) {
		manifests := GetAllManifests()
		for _, m := range manifests {
			if !args["--full"].(bool) {
				m.Readme = ""
				m.ChangeLog = ""
			}
			yml, _ := yaml.Marshal(m)
			fmt.Println(string(yml))
		}
		os.Exit(0)

	} else if args["sync"].(bool) {
		msg := `Clone and/or Update all Bench repositories?
			Existing work will be stashed and pull the master branch.
			Please make sure you are in the directory where you
			store your repos and not a specific repo.`

		if args["--force"].(bool) || askForConfirmation(msg) {
			SyncRepositories(GetAllManifests())
		}
		os.Exit(0)

	} else if args["setup"].(bool) {
		Setup()

	} else if args["ec2"].(bool) {
		name := args["INSTANCE_NAME"]
		command := []string{"-tC"}
		if len(args["COMMAND"].([]string)) > 0 {
			cmd := args["COMMAND"].([]string)
			switch cmd[0] {
			case "exec":
				command = append(append(command, "/opt/bench/exec"), cmd[1:]...)
			case "jstack":
				command = append(append(command, "/opt/bench/jstack"), cmd[1:]...)
			case "jmap":
				command = append(append(command, "/opt/bench/jmap"), cmd[1:]...)
			default:
				command = append(command, cmd...)
			}
		}
		if name != nil {
			ConnectToInstance(name.(string), command...)
		} else {
			ConnectToInstance("")
		}
		os.Exit(0)

	} else if args["eb"].(bool) && args["events"].(bool) {
		ListEvents()
		os.Exit(0)

	} else if args["eb"].(bool) {
		ListEnvironments()
		os.Exit(0)
	}

	m := BuildManifest(args["--artifact-version"].(string))

	if args["validate"].(bool) {
		//TODO: Build proper validation
		yml, _ := yaml.Marshal(m)
		log.Println(string(yml))

	} else if args["update"].(bool) {
		StoreManifest(m)

	} else if args["repo"].(bool) {
		OpenGH(m, "")

	} else if args["issues"].(bool) {
		OpenGH(m, "issues")

	} else if args["branches"].(bool) {
		OpenGH(m, "branches")

	} else if args["pr"].(bool) {
		OpenGH(m, "pulls")

	} else if args["raml"].(bool) {
		base := "https://github.com/BenchLabs/bench-raml/tree/master/specs/"
		OpenURI(base + m.Repository + ".raml")

	} else if args["jenkins"].(bool) && args["console"].(bool) {
		OpenJenkins(m, "job/master/lastBuild/consoleFull")

	} else if args["jenkins"].(bool) && args["trigger"].(bool) {
		OpenJenkins(m, "job/master/trigger")

	} else if args["jenkins"].(bool) {
		OpenJenkins(m, "")

	} else if args["splunk"].(bool) {
		OpenSplunk(m, args["staging"].(bool))

	} else if args["docs"].(bool) {
		base := "https://example.atlassian.net/wiki/display/dev/"
		OpenURI(base + m.Name)

	} else if args["circle"].(bool) {
		OpenCircle(m, args["branch"].(bool))
	}
}
