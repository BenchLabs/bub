package ci

import (
	"errors"
	"fmt"
	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/utils"
	"github.com/jszwedko/go-circleci"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	NoBuildFound = errors.New("no build found for the commit")
)

type Circle struct {
	cfg    *core.Configuration
	client *circleci.Client
}

func MustInitCircle(cfg *core.Configuration) *Circle {
	token := os.Getenv("CIRCLE_TOKEN")
	if token == "" && cfg.Circle.Token == "" {
		log.Fatal("Please set the CircleCI token in your keychain or set with the CIRCLE_TOKEN environment variable.")
	} else if cfg.Circle.Token != "" {
		token = cfg.Circle.Token
	}
	return &Circle{cfg, &circleci.Client{Token: token}}
}

func OpenCircle(cfg *core.Configuration, m *core.Manifest, getBranch bool) error {
	base := "https://circleci.com/gh/" + cfg.GitHub.Organization
	if getBranch {
		currentBranch := url.QueryEscape(core.InitGit().GetCurrentBranch())
		return utils.OpenURI(base, m.Repository, "tree", currentBranch)
	}
	return utils.OpenURI(base, m.Repository)
}

func (c *Circle) TriggerAndWaitForSuccess(m *core.Manifest) error {
	b, err := c.client.Build(c.cfg.GitHub.Organization, m.Repository, m.Branch)
	if err != nil {
		return err
	}

	log.Printf("Triggered b: %s", b.BuildURL)
	time.Sleep(1 * time.Second)

	for {
		b, err = c.client.GetBuild(c.cfg.GitHub.Organization, m.Repository, b.BuildNum)
		if err != nil {
			return err
		}

		if isFinished(b) {
			break
		}
		log.Printf("Current lifecycle state: %s, waiting 20s...", b.Lifecycle)
		time.Sleep(20 * time.Second)
	}
	return isSuccess(b)
}

func isSuccess(b *circleci.Build) error {
	if b.Outcome == "success" {
		log.Printf("The build succeeded! %v", b.BuildURL)
		return nil
	} else {
		return errors.New(fmt.Sprintf("the build failed: %s, %s", b.Outcome, b.BuildURL))
	}
}

func isFinished(build *circleci.Build) bool {
	return utils.Contains(build.Lifecycle, "finished", "not_run")
}

func configurationExist() (bool, error) {
	legacyConfiguration, err := utils.PathExists("circle.yml")
	if err != nil {
		return false, err
	}
	configuration, err := utils.PathExists(".circleci")
	if err != nil {
		return false, err
	}
	return legacyConfiguration || configuration, nil
}

func (c *Circle) CheckBuildStatus(m *core.Manifest) error {
	exists, err := configurationExist()
	if err != nil {
		return err
	}
	if !exists {
		log.Printf("CircleCI not configured. Skipping check...")
		return nil
	}
	p, err := c.client.FollowProject(c.cfg.GitHub.Organization, m.Repository)
	if err != nil && !strings.HasPrefix(err.Error(), "403") {
		log.Printf("failed to follow circleci project: %v", err)
		return err
	} else if p == nil {
		if err != nil {
			log.Printf("API Error: %v", err)
		}
		log.Printf("CircleCI not configured or the current user has no access to the project. Skipping check...")
		return nil
	}
	head, err := core.MustInitGit(".").CurrentHEAD()
	if err != nil {
		log.Printf("failed to initialize git: %v", err)
		return err
	}
	log.Printf("Commit: %v", head)
	var b *circleci.Build
	for {
		b, err = c.checkBuildStatus(head, m)
		if err != nil {
			log.Printf("failed to check circleci build status: %v", err)
			return err
		}
		if isFinished(b) {
			break
		}
		log.Printf("Status: '%v', waiting 10s. %v", b.Status, b.BuildURL)
		time.Sleep(10 * time.Second)
	}

	return isSuccess(b)
}

func (c *Circle) checkBuildStatus(head string, m *core.Manifest) (*circleci.Build, error) {
	builds, err := c.client.ListRecentBuildsForProject(c.cfg.GitHub.Organization, m.Repository, m.Branch, "", 50, 0)
	if err != nil {
		return nil, err
	}
	for _, b := range builds {
		commit := b.AllCommitDetails[len(b.AllCommitDetails)-1].Commit
		if commit == head {
			return b, nil
		}
	}
	return nil, NoBuildFound
}
