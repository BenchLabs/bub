package main

import (
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

func GetCurrentRepositoryName() string {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	result, err := cmd.Output()

	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	repositoryUri := string(result)
	return strings.TrimSuffix(path.Base(repositoryUri), path.Ext(repositoryUri))
}

func GetCurrentBranch() string {
	result, err := exec.Command("git", "symbolic-ref", "--short", "-q", "HEAD").Output()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	return strings.Trim(string(result), "\n ")
}

func IsInRepository() bool {
	result, err := exists(".git")
	if err != nil {
		return false
	}
	return result
}

func CloneRepository(repository string) {
	log.Printf("Cloning: %v", repository)
	runCmd("git", "clone", "git@github.com:"+repository+".git")
}

func UpdateRepository(repository string) {
	log.Printf("Updating: %v", repository)
	dir, _ := os.Getwd()
	os.Chdir(path.Join(dir, repository))
	runCmd("git", "stash")
	runCmd("git", "checkout", "master")
	runCmd("git", "pull")
	os.Chdir(dir)
}

func SyncRepositories(ms []Manifest) {
	for _, m := range ms {
		syncRepository(m)
	}
}

func syncRepository(m Manifest) {
	repository := m.Repository
	repositoryExists, _ := exists(repository)
	if repositoryExists {
		UpdateRepository(repository)
	} else {
		CloneRepository(repository)
	}
}

func runCmd(cmd string, args ...string) {
	err := exec.Command(cmd, args...).Run()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

