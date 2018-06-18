package utils

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

var (
	FileDoesNotExist = errors.New("file or directory does not exists")
)

func CurrentTimeForFilename() string {
	return time.Now().Format("2006-01-02T15-04-05Z")
}

func GetEnvWithDefault(key string, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		val = defaultValue
	}
	return val
}

func PathExists(filePath ...string) (bool, error) {
	_, err := os.Stat(path.Join(filePath...))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

// Copy the src file to dst. Any existing file will be overwritten and will not
// Copy file attributes.
// From https://stackoverflow.com/questions/21060945/simple-way-to-copy-a-file-in-golang
func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func EditFile(filePath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	editorCmd := strings.Split(editor, " ")
	editorArgs := append(editorCmd[1:], filePath)
	cmd := exec.Command(editorCmd[0], editorArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CreateAndEdit(filePath string, content string) error {
	directory := path.Dir(filePath)
	dirExists, err := PathExists(directory)
	if err != nil {
		log.Fatal(err)
	}

	if !dirExists {
		os.MkdirAll(directory, 0700)
	}

	fileExists, err := PathExists(filePath)
	if err != nil {
		log.Fatal(err)
	}

	if !fileExists {
		log.Printf("Creating %s file.\n", filePath)
		ioutil.WriteFile(filePath, []byte(content), 0700)
	}

	log.Printf("Editing %s.", filePath)
	return EditFile(filePath)
}

func JoinStringPointers(ptrs []*string, joinStr string) string {
	var arr []string
	for _, ref := range ptrs {
		if ref == nil {
			arr = append(arr, "")
		} else {
			arr = append(arr, *ref)
		}
	}
	return strings.Join(arr, joinStr)
}

func PickItem(label string, items []string) (string, error) {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}:",
		Active:   "▶ {{ . }}",
		Inactive: "  {{ . }}",
		Selected: "▶ {{ . }}",
	}

	searcher := func(input string, index int) bool {
		i := items[index]
		name := strings.Replace(strings.ToLower(i), " ", "", -1)
		input = strings.Replace(strings.ToLower(input), " ", "", -1)
		return strings.Contains(name, input)
	}

	prompt := promptui.Select{
		Size:              20,
		Label:             label,
		Items:             items,
		Templates:         templates,
		Searcher:          searcher,
		StartInSearchMode: true,
	}

	i, _, err := prompt.Run()
	return items[i], err
}

func Random(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

func HasNonEmptyLines(lines []string) bool {
	for _, s := range lines {
		if s != "" {
			return true
		}
	}
	return false
}

func ConditionalOp(message string, noop bool, fn func() error) error {
	if noop {
		log.Printf("%v (noop)", message)
		return nil
	}
	log.Printf(message)
	return fn()
}

func InRepository() bool {
	return IsRepository(".")
}

func IsRepository(repoDir string) bool {
	return exec.Command("git", "-C", repoDir, "status").Run() == nil
}

func OpenURI(uriSegments ...string) error {
	uri := strings.Join(uriSegments, "/")
	log.Printf("Opening: %v", uri)
	if runtime.GOOS == "darwin" {
		return exec.Command("open", uri).Run()
	} else if runtime.GOOS == "linux" {
		return exec.Command("xdg-open", uri).Run()
	}
	return errors.New("could not open the link automatically")
}

func RemoveDuplicatesUnordered(elements []string) []string {
	encountered := map[string]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		encountered[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	result := []string{}
	for key := range encountered {
		result = append(result, key)
	}
	return result
}

// TimeTrack measure the excution time of a method
// func factorial(n *big.Int) (result *big.Int) {
//		// defer timeTrack(time.Now(), "factorial")
//		// ... do some things, maybe even return under some condition
//		return n
// }
func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

// Contains returns a boolean whether a string is contained in a series of strings
func Contains(str string, haystack ...string) bool {
	for _, i := range haystack {
		if str == i {
			return true
		}
	}
	return false
}

func DownloadFile(path string, url string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}
