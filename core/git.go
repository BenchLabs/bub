package core

import (
	"fmt"
	"github.com/benchlabs/bub/utils"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"text/tabwriter"
)

type Git struct {
	cfg *Configuration
	dir string
}

type GitCommit struct {
	Hash, Committer, Subject, Body string
}

type RepoOperation func(string) (string, error)

func InitGit() *Git {
	return &Git{}
}

func MustInitGit(repoDir string) *Git {
	if repoDir == "" {
		repoDir = "."
	}
	return &Git{dir: repoDir}
}

func (g *Git) RunGit(args ...string) error {
	if g.dir != "" {
		args = append([]string{"-C", g.dir}, args...)
	}
	log.Printf("Running: 'git %v'", strings.Join(args, " "))
	return utils.RunCmd("git", args...)
}

func (g *Git) RunGitWithStdout(args ...string) (string, error) {
	if g.dir != "" {
		args = append([]string{"-C", g.dir}, args...)
	}
	return utils.RunCmdWithStdout("git", args...)
}

func (g *Git) RunGitWithFullOutput(args ...string) (string, error) {
	if g.dir != "" {
		args = append([]string{"-C", g.dir}, args...)
	}
	return utils.RunCmdWithFullOutput("git", args...)
}

func (g *Git) MustRunGitWithStdout(args ...string) string {
	output, err := g.RunGitWithStdout(args...)
	if err != nil {
		log.Fatalf("Git failed: %v", err)
	}
	return output
}

func (g *Git) GetCurrentRepositoryName() string {
	repositoryUri := g.MustRunGitWithStdout("config", "--get", "remote.origin.url")
	return strings.TrimSuffix(path.Base(repositoryUri), path.Ext(repositoryUri))
}

func (g *Git) GetCurrentBranch() string {
	result, err := g.RunGitWithStdout("symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		// if on jenkins the HEAD is usually detached, but you can infer the branch name.
		branchEnv := os.Getenv("BRANCH_NAME")
		if branchEnv != "" {
			log.Printf("Could not get branch name from git: %v", err)
			log.Printf("Inferring from environment variables: %v", branchEnv)
		}
		return branchEnv
	}

	return strings.Trim(string(result), "\n ")
}

func (g *Git) GetRepositoryRootPath() (string, error) {
	return g.RunGitWithStdout("rev-parse", "--show-toplevel")
}

func (g *Git) GetTitleFromBranchName() string {
	branch := g.GetCurrentBranch()
	return strings.Replace(strings.Replace(strings.Replace(branch, "-", "_", 1), "-", " ", -1), "_", "-", -1)
}

func (g *Git) Clone() (string, error) {
	log.Printf("Cloning: %v", g.dir)
	return utils.RunCmdWithFullOutput("git", "clone", "git@github.com:benchlabs/"+g.dir+".git")
}

func (g *Git) Push(cfg *Configuration) error {
	args := []string{"push", "--set-upstream", "origin", g.GetCurrentBranch()}
	if cfg.Git.NoVerify {
		args = append(args, "--no-verify")
	}
	return g.RunGit(args...)
}

func (g *Git) Sync(unStash bool) (string, error) {
	commands := [][]string{
		{"reset", "HEAD", g.dir},
	}
	dirtyTree := g.RunGit("diff-index", "--quiet", "HEAD", "--") != nil
	if dirtyTree {
		commands = append(commands, [][]string{
			{"checkout", "master", "-f"},
			{"stash", "save", "pre-update-" + utils.CurrentTimeForFilename()},
		}...)
	}
	commands = append(commands, [][]string{
		{"checkout", "master", "-f"},
		{"clean", "-fd"},
		{"checkout", "master", "."},
		{"pull"},
		{"pull", "--tags"},
	}...)
	if dirtyTree && unStash {
		commands = append(commands, []string{"stash", "apply"})
	}
	for _, cmd := range commands {
		out, err := g.RunGitWithFullOutput(cmd...)
		if err != nil {
			return out, err
		}
	}
	return "", nil
}

func SyncRepositories() error {
	manifests := GetManifestRepository().GetAllActiveManifests()
	var repos []string
	for _, m := range manifests {
		repos = append(repos, m.Repository)
	}
	return ConcurrentRepositoryOperations(repos, func(repo string) (string, error) {
		return MustInitGit(repo).syncRepository()
	})
}

type ConcurrentResult struct {
	Output string
	Err    error
}
type ConcurrentResults map[string]ConcurrentResult

func ConcurrentRepositoryOperations(repos []string, fn RepoOperation) error {
	var wg sync.WaitGroup
	var mutex sync.Mutex
	errs := ConcurrentResults{}
	for _, r := range repos {
		log.Printf("Sync: %v", r)
		wg.Add(1)
		go func(repo string) {
			defer wg.Done()
			output, err := fn(repo)
			mutex.Lock()
			errs[repo] = ConcurrentResult{Output: output, Err: err}
			mutex.Unlock()
			log.Printf("%v: done.", repo)
		}(r)
	}
	wg.Wait()
	errorCount := 0
	for repo, result := range errs {
		fmt.Println(result.Output)
		if result.Err != nil {
			errorCount++
			log.Printf("%v failed to be updated: %v", repo, result.Err)
		}
	}
	if errorCount > 0 {
		log.Printf("%v repos failed to be updated.", errorCount)
		return errors.New("some repos failed to update")
	}
	log.Print("All Done.")
	return nil
}

func (g *Git) syncRepository() (string, error) {
	repositoryExists, _ := utils.PathExists(g.dir)
	if repositoryExists {
		return g.Sync(true)
	} else {
		return g.Clone()
	}
}

func (g *Git) Log() (commits []*GitCommit) {
	output := strings.Split(g.MustRunGitWithStdout("log", "--pretty=format:%h||~||%an||~||%s||~||%b|~~~~~|"), "|~~~~~|\n")
	for _, line := range output {
		if len(line) == 0 {
			continue
		}
		fields := strings.Split(line, "||~||")
		commits = append(commits, &GitCommit{Hash: fields[0], Committer: fields[1], Subject: fields[2], Body: fields[3]})
	}
	return commits
}

func (g *Git) PendingChanges(cfg *Configuration, manifest *Manifest, previousVersion, currentVersion string, formatForSlack bool, noAt bool) {
	table := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	output := g.MustRunGitWithStdout("log", "--first-parent", "--pretty=format:%h\t\t%an\t%s", previousVersion+"..."+currentVersion)
	if formatForSlack {
		re := g.GetIssueRegex()
		output = re.ReplaceAllString(output, "<https://"+cfg.JIRA.Server+"/browse/$1|$1>")
		re = g.GetPRRegex()
		output = re.ReplaceAllString(output, "<https://github.com/"+cfg.GitHub.Organization+"/"+manifest.Repository+"/pull/$2|PR#$2> ")
		re = regexp.MustCompile("(?m:^)([a-z0-9]{6,})")
		output = re.ReplaceAllString(output, "<https://github.com/"+cfg.GitHub.Organization+"/"+manifest.Repository+"/commit/$1|$1>")
	}
	fmt.Fprintln(table, output)
	table.Flush()
	if !noAt {
		committerSlackArr := g.committerSlackReference(cfg, previousVersion, currentVersion)
		if formatForSlack {
			fmt.Print("\n" + strings.Join(committerSlackArr, ", "))
		}
	}
}
func (g *Git) GetPRRegex() *regexp.Regexp {
	return regexp.MustCompile("(Merge pull request #)(\\d+) from \\w+/")
}
func (g *Git) GetIssueRegex() *regexp.Regexp {
	return regexp.MustCompile("([A-Z]{2,}-\\d+)")
}

func (g *Git) PickCommit(commits []*GitCommit) (*GitCommit, error) {
	templates := &promptui.SelectTemplates{
		Label: "{{ . }}:",
		Active: "▶ {{ .Hash }}	{{ .Subject }}",
		Inactive: "  {{ .Hash }}	{{ .Subject }}",
		Selected: "▶ {{ .Hash }}	{{ .Subject }}",
		Details: `
{{ .Hash }}
{{ .Committer }}
{{ .Subject }}
{{ .Body }}
`,
	}

	searcher := func(input string, index int) bool {
		i := commits[index]
		name := strings.Replace(strings.ToLower(i.Subject), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Size:              20,
		Label:             "Pick commit",
		Items:             commits,
		Templates:         templates,
		Searcher:          searcher,
		StartInSearchMode: true,
	}
	i, _, err := prompt.Run()
	return commits[i], err
}

func (g *Git) FetchTags() error {
	return g.RunGit("fetch", "--tags")
}

func (g *Git) Fetch() error {
	return g.RunGit("fetch")
}

func (g *Git) sanitizeBranchName(name string) string {
	r := regexp.MustCompile("[^a-zA-Z0-9]+")
	r2 := regexp.MustCompile("-+")
	return strings.Trim(r2.ReplaceAllString(r.ReplaceAllString(name, "-"), "-"), "-")
}

func (g *Git) LogNotInMainSubjects() []string {
	return strings.Split(g.MustRunGitWithStdout("log", "HEAD", "--not", "origin/master", "--no-merges", "--pretty=format:%s"), "\n")
}

func (g *Git) LogNotInMainBody() string {
	return g.MustRunGitWithStdout("log", "HEAD", "--not", "origin/master", "--no-merges", "--pretty=format:-> %B")
}

func (g *Git) ListFileChanged() []string {
	return strings.Split(g.MustRunGitWithStdout("diff", "HEAD", "--not", "origin/master", "--name-only"), "\n")
}

func (g *Git) GetIssueKeyFromBranch() string {
	return g.extractIssueKeyFromName(g.GetCurrentBranch())
}

func (g *Git) CommitWithBranchName() error {
	return g.RunGit("commit", "-m", g.GetTitleFromBranchName(), "--all")
}

func (g *Git) CurrentHEAD() (string, error) {
	return g.RunGitWithStdout("rev-parse", "HEAD")
}

func (g *Git) CommitWithIssueKey(cfg *Configuration, message string, extraArgs []string) error {
	issueKey := g.GetIssueKeyFromBranch()
	if message == "" {
		title := g.GetTitleFromBranchName()
		pos := strings.Index(title, " ")
		if pos < 0 {
			return errors.New("commit message could not be inferred from branch name")
		}
		message = title[pos:]
	}
	message = strings.Trim(message, " ")
	if len(message) == 0 {
		return errors.New("no commit message passed or could not be inferred from branch name")
	}
	if issueKey != "" {
		message = issueKey + " " + message
	}
	args := []string{
		"commit", "-m", message,
	}
	if cfg.Git.NoVerify {
		args = append(args, "--no-verify")
	}
	args = append(args, extraArgs...)
	return g.RunGit(args...)
}

func (g *Git) extractIssueKeyFromName(name string) string {
	return g.GetIssueRegex().FindString(name)
}

func (g *Git) CreateBranch(name string) error {
	name = g.sanitizeBranchName(name)
	return g.RunGit("checkout", "-b", name, "origin/master")
}

func (g *Git) ForceCreateBranch(name string) error {
	name = g.sanitizeBranchName(name)
	return g.RunGit("checkout", "-B", name, "origin/master")
}

func (g *Git) CheckoutBranch() error {
	item, err := utils.PickItem("Pick a branch", g.getBranches())
	if err != nil {
		return err
	}
	return g.RunGit("checkout", item)
}

func ForEachRepo(fn RepoOperation) error {
	var repos []string
	files, err := ioutil.ReadDir("./")
	if err != nil {
		return err
	}
	for _, value := range files {
		if !value.IsDir() {
			continue
		}
		if !utils.IsRepository(value.Name()) {
			continue
		}
		repos = append(repos, value.Name())
	}
	return ConcurrentRepositoryOperations(repos, fn)
}

func (g *Git) getBranches() []string {
	output := g.MustRunGitWithStdout("branch", "--all", "--sort=-committerdate")
	var branches []string
	for _, b := range strings.Split(output, "\n") {
		b = strings.TrimPrefix(strings.Trim(b, " "), "* ")
		if b == "" {
			continue
		}
		branches = append(branches, b)
	}
	return branches
}

func (g *Git) committerSlackReference(cfg *Configuration, previousVersion string, currentVersion string) []string {
	committerMapping := make(map[string]string)
	for _, i := range cfg.Users {
		committerMapping[i.Name] = i.Slack
	}

	committersStdout := g.MustRunGitWithStdout("log", "--first-parent", "--pretty=format:%an", previousVersion+"..."+currentVersion)
	committersSlackMapping := make(map[string]string)
	for _, commiterName := range strings.Split(committersStdout, "\n") {
		slackUserName := committerMapping[commiterName]
		if slackUserName == "" {
			slackUserName = commiterName
		} else {
			slackUserName = "@" + slackUserName
		}
		committersSlackMapping[commiterName] = slackUserName
	}

	var committerSlackArr []string
	for _, v := range committersSlackMapping {
		committerSlackArr = append(committerSlackArr, v)
	}
	return committerSlackArr
}

func (g *Git) ContainedUncommittedChanges() bool {
	return utils.HasNonEmptyLines(strings.Split(g.MustRunGitWithStdout("status", "--short"), "\n"))
}

func (g *Git) IsDifferentFromMainBranch() bool {
	return utils.HasNonEmptyLines(g.LogNotInMainSubjects())
}

func (g *Git) ISDirty() bool {
	return g.RunGit("diff-index", "--quiet", "HEAD", "--") != nil
}

func (g *Git) Diff() (string, error) {
	if !g.IsDifferentFromMainBranch() {
		return "", nil
	}
	return g.RunGitWithFullOutput("--no-pager", "diff")
}
