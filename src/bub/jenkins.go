package main

import (
	"fmt"
	"github.com/bndr/gojenkins"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"
)

func GetJobName(m Manifest) string {
	return strings.Join([]string{"BenchLabs", "job", m.Repository, "job", m.Branch}, "/")
}

func GetClient(cfg Configuration) *gojenkins.Jenkins {
	if cfg.Jenkins.Server == "" {
		log.Fatal("server cannot be empty, make sure the config file is properly configured. run 'bub config'.")
	}
	if strings.HasPrefix(cfg.Jenkins.Username, "<") ||
		cfg.Jenkins.Username == "" || cfg.Jenkins.Password == "" {
		log.Fatal("please set your jenkins credentials. run 'bub config'.")
	}
	client, err := gojenkins.CreateJenkins(cfg.Jenkins.Server, cfg.Jenkins.Username, cfg.Jenkins.Password).Init()
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func GetJob(cfg Configuration, m Manifest) *gojenkins.Job {
	client := GetClient(cfg)
	uri := GetJobName(m)
	job, err := client.GetJob(uri)
	if err != nil {
		log.Fatalf("failed to fetch job details. error: %s", err)
	}
	return job
}

func GetLastBuild(cfg Configuration, m Manifest) *gojenkins.Build {
	log.Printf("fetching last build for '%v' '%v'.", m.Repository, m.Branch)
	lastBuild, err := GetJob(cfg, m).GetLastBuild()
	if err != nil {
		log.Fatalf("failed to fetch build details. error: %s", err)
	}
	log.Printf(lastBuild.GetUrl())
	return lastBuild
}

func GetArtifacts(cfg Configuration, m Manifest) {
	log.Print("fetching artifacts.")
	artifacts := GetLastBuild(cfg, m).GetArtifacts()
	dir, _ := ioutil.TempDir("", strings.Join([]string{m.Repository, m.Branch}, "-"))
	for _, artifact := range artifacts {
		if !strings.Contains(artifact.FileName, ".png") {
			artifactPath := path.Join(dir, artifact.FileName)
			log.Println(artifactPath)
			artifact.Save(artifactPath)
		} else {
			log.Println(cfg.Jenkins.Server + artifact.Path)
		}
	}
}

func ShowConsoleOutput(cfg Configuration, m Manifest) {
	var lastChar int
	for {
		build, err := GetJob(cfg, m).GetLastBuild()
		if lastChar == 0 {
			log.Print(build.GetUrl())
		}
		if err != nil {
			log.Fatalf("could not find the last build. make sure it was triggered at least once", err)
		}
		consoleOutput := build.GetConsoleOutput()
		for i, char := range consoleOutput {
			if i > lastChar {
				fmt.Print(string(char))
			}
		}
		lastChar = len(consoleOutput) - 1
		if !build.IsRunning() {
			if !build.IsGood() {
				log.Fatal("the job failed on jenkins.")
			}
			break
		}
		time.Sleep(2 * time.Second)
	}
}

func BuildJob(cfg Configuration, m Manifest, async bool, force bool) {
	jobName := GetJobName(m)
	job := GetJob(cfg, m)
	lastBuild, err := job.GetLastBuild()
	if err == nil && lastBuild.IsRunning() && !force {
		log.Fatal("a build for this job is already running pass '--force' to trigger the build.")
	} else if err != nil && err.Error() != "404" {
		log.Fatalf("failed to get last build status: %v", err)
	}

	job.InvokeSimple(nil)
	log.Printf("build triggered: %v/job/%v wating for the job to start.", cfg.Jenkins.Server, jobName)

	if async {
		return
	}

	for {
		newBuild, err := GetJob(cfg, m).GetLastBuild()
		if err == nil && (lastBuild == nil || (lastBuild.GetUrl() != newBuild.GetUrl())) {
			os.Stderr.WriteString("\n")
			break
		} else if err != nil && err.Error() != "404" {
			log.Fatalf("failed to get build status: %v", err)
		}
		os.Stderr.WriteString(".")
		time.Sleep(2 * time.Second)
	}
	ShowConsoleOutput(cfg, m)
}
