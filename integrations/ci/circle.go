package ci

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/benchlabs/bub/core"
	"github.com/benchlabs/bub/utils"
	"github.com/jszwedko/go-circleci"
)

var (
	NoBuildFound = errors.New("no build found for the commit")
)

type CircleCINotConfiguredError struct {
	msg string
}

func (ce *CircleCINotConfiguredError) Error() string {
	return fmt.Sprintf("CircleCI Configuration Error : %s", ce.msg)
}

func NewCircleCINotConfiguredError(s string) *CircleCINotConfiguredError {
	return &CircleCINotConfiguredError{s}
}

type Circle struct {
	cfg    *core.Configuration
	client *circleci.Client
}

func (c *Circle) GetAccount() string {
	return c.cfg.GitHub.Organization
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
	b, err := c.GetCompletedBuild(m)
	if err != nil {
		if _, ok := err.(*CircleCINotConfiguredError); ok {
			return nil
		}
		return err
	}
	return isSuccess(b)
}

func (c *Circle) GetCompletedBuild(m *core.Manifest) (*circleci.Build, error) {
	var build *circleci.Build
	exists, err := configurationExist()
	if err != nil {
		return build, err
	}
	if !exists {
		errMsg := "CircleCI not configured. Skipping check..."
		log.Printf(errMsg)
		return build, NewCircleCINotConfiguredError(errMsg)
	}
	p, err := c.client.FollowProject(c.cfg.GitHub.Organization, m.Repository)
	if err != nil && !strings.HasPrefix(err.Error(), "403") {
		return build, err
	} else if p == nil {
		if err != nil {
			log.Printf("API Error: %v", err)
		}
		errMsg := "CircleCI not configured or the current user has no access to the project. Skipping check..."
		log.Printf(errMsg)
		return build, NewCircleCINotConfiguredError(errMsg)
	}
	head, err := core.MustInitGit(".").CurrentHEAD()
	if err != nil {
		return build, err
	}
	log.Printf("Commit: %v", head)
	for {
		build, err = c.checkBuildStatus(head, m)
		if err != nil {
			return build, err
		}
		if isFinished(build) {
			break
		}
		log.Printf("Status: '%v', waiting 10s. %v", build.Status, build.BuildURL)
		time.Sleep(10 * time.Second)
	}
	return build, nil
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

func (c *Circle) DownloadArtifact(m *core.Manifest, fname, path string) error {
	b, err := c.GetCompletedBuild(m)
	if err != nil {
		return err
	}

	arr, err := getBuildArtifacts(c.client, c.GetAccount(), m.Repository, b)
	if err != nil {
		return err
	}

	for _, a := range arr {
		if strings.Contains(a.URL, fname) {
			return utils.DownloadFile(path, fmt.Sprintf("%s?circle-token=%s", a.URL, c.client.Token))
		}
	}

	return nil
}

func getBuildArtifacts(c *circleci.Client, account, repo string, b *circleci.Build) ([]*circleci.Artifact, error) {
	var arr []*circleci.Artifact
	arr, err := c.ListBuildArtifacts(account, repo, b.BuildNum)
	if err != nil {
		return arr, err
	}
	return arr, nil
}
