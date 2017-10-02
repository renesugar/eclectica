package python

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/blang/semver"
	"github.com/chuckpreslar/emission"
	"github.com/go-errors/errors"
	"github.com/kr/pty"
	"gopkg.in/cavaliercoder/grab.v1"

	"github.com/markelog/eclectica/console"
	"github.com/markelog/eclectica/pkg"
	"github.com/markelog/eclectica/plugins/python/patch"
	"github.com/markelog/eclectica/variables"
	"github.com/markelog/eclectica/versions"
)

var (
	VersionLink    = "https://hg.python.org/cpython/tags"
	remoteVersion  = "https://www.python.org/ftp/python"
	versionPattern = "^\\d+\\.\\d+(?:\\.\\d)?"

	pipName = "get-pip.py"
	baseUrl = "https://bootstrap.pypa.io/"
	pipUrl  = baseUrl + pipName

	// Hats off to inconsistent python developers
	noNilVersions, _ = semver.Make("3.3.0")
	// When pip began to be available with binaries
	pipAvailable, _ = semver.Make("2.7.9")

	bins = []string{"2to3", "idle", "pydoc", "python", "python-config", "pip", "easy_install"}
	dots = []string{".python-version"}
)

type Python struct {
	Version string
	Emitter *emission.Emitter
	pkg.Base
}

func New(version string, emitter *emission.Emitter) *Python {
	return &Python{
		Version: version,
		Emitter: emitter,
	}
}

func (python Python) Events() *emission.Emitter {
	return python.Emitter
}

func (python Python) PreInstall() error {
	if runtime.GOOS == "linux" {
		return dealWithLinuxShell()
	}

	return dealWithOSXShell()
}

func (python Python) Install() (err error) {
	err = python.configure()
	if err != nil {
		return
	}

	err = python.prepare()
	if err != nil {
		return
	}

	err = python.install()
	if err != nil {
		return
	}

	return python.renameLinks()
}

func (python Python) PostInstall() (err error) {
	path := variables.Path("python", python.Version)
	bin := variables.GetBin("python", python.Version)

	if hasTools(python.Version) {
		err, cmd, stderr, stdout := python.getCmd(bin, "-m", "ensurepip")
		if err != nil {
			return err
		}

		err = cmd.Run()
		if err != nil {
			return console.GetError(err, stderr, stdout)
		}

		return err
	}

	// Setup pip
	pip := filepath.Join(path, pipName)
	out, err := exec.Command(bin, pip).CombinedOutput()
	if err != nil {
		return errors.New(string(out))
	}

	// Install setuptools with "pip" apparently, this is best way to do it
	pipBin := filepath.Join(variables.Path("python", python.Version), "bin", "pip")
	out, err = exec.Command(pipBin, "install", "setuptools", "--upgrade").CombinedOutput()
	if err != nil {
		return errors.New(string(out))
	}

	return nil
}

func (python Python) Info() map[string]string {
	var (
		result    = make(map[string]string)
		version   = python.Version
		chosen, _ = semver.Make(python.Version)

		patch = strconv.Itoa(int(chosen.Patch))
		minor = strconv.Itoa(int(chosen.Minor))
		major = strconv.Itoa(int(chosen.Major))

		urlPart = major + "." + minor + "." + patch
	)

	// Hats off to inconsistent python developers
	if chosen.LT(noNilVersions) {
		version = versions.Unsemverify(version)
		version = strings.Replace(version, "-", "", 1)

		urlPart = versions.Unsemverify(urlPart)
	}

	// Python 2.0 has different format and it's not supported
	result["extension"] = "tgz"
	result["version"] = version
	result["filename"] = "Python-" + version
	result["url"] = fmt.Sprintf(
		"%s/%s/%s.%s",
		remoteVersion,
		urlPart,
		result["filename"],
		result["extension"],
	)

	return result
}

func (rust Python) Bins() []string {
	return bins
}

func (rust Python) Dots() []string {
	return dots
}

func (python Python) ListRemote() (result []string, err error) {
	doc, err := goquery.NewDocument(VersionLink)

	if err != nil {
		if _, ok := err.(net.Error); ok {
			return nil, errors.New(variables.ConnectionError)
		}

		return nil, errors.New(err)
	}

	tmp := []string{}
	version := regexp.MustCompile(versionPattern)
	links := doc.Find(".bigtable td:first-child a")

	for i := range links.Nodes {
		content := links.Eq(i).Text()

		content = strings.TrimSpace(content)
		content = strings.Replace(content, "v", "", 1)

		if version.MatchString(content) {
			tmp = append(tmp, content)
		}
	}

	// Remove < 2.7 versions and "Pre" versions
	for _, element := range tmp {
		smr, _ := semver.Make(versions.Semverify(element))

		if len(smr.Pre) > 0 {
			continue
		}
		if smr.Major < 2 {
			continue
		}
		if smr.Major == 2 && smr.Minor < 7 {
			continue
		}

		result = append(result, element)
	}

	return
}

func (python Python) configure() (err error) {
	python.Emitter.Emit("configure")

	path := variables.Path("python", python.Version)
	configure := filepath.Join(path, "configure")

	err = python.externals()
	if err != nil {
		return errors.New(err)
	}

	err, cmd, stderr, stdout := python.getCmd(configure, "--prefix="+path, "--with-ensurepip=upgrade")
	if err != nil {
		return err
	}
	cmd.Env = python.getEnvs(cmd.Env)

	python.listen("configure", stdout)

	err = cmd.Run()
	if err != nil {
		return console.GetError(err, stderr, stdout)
	}

	return
}

func (python Python) prepare() (err error) {
	python.Emitter.Emit("prepare")

	// Ignore touch errors since newest python makefile doesn't have this task
	python.touch()

	err, cmd, stderr, stdout := python.getCmd("make", "-j", "2")
	if err != nil {
		return err
	}

	python.listen("prepare", stdout)

	err = cmd.Run()
	if err != nil {
		return console.GetError(err, stderr, stdout)
	}

	return
}

func (python Python) install() (err error) {
	python.Emitter.Emit("install")

	err, cmd, stderr, stdout := python.getCmd("make", "install")
	if err != nil {
		return err
	}

	python.listen("install", stdout)

	err = cmd.Run()
	if err != nil {
		return console.GetError(err, stderr, stdout)
	}

	return
}

func (python Python) touch() (err error) {
	err, cmd, stderr, stdout := python.getCmd("make", "touch")
	if err != nil {
		return err
	}

	err = cmd.Run()
	if err != nil {
		return console.GetError(err, stderr, stdout)
	}

	return
}

func truncateString(str string, num int) string {
	bnoden := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		bnoden = str[0:num] + "..."
	}
	return bnoden
}

func (python Python) listen(event string, pipe io.ReadCloser) {
	if pipe == nil {
		return
	}

	scanner := bufio.NewScanner(pipe)
	go func() {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if len(line) == 0 {
				continue
			}

			line = truncateString(line, 80)

			python.Emitter.Emit(event, line)
		}
	}()
}

func (python Python) getEnvs(original []string) (result []string) {
	if runtime.GOOS == "darwin" {
		result = python.getOSXEnvs(original)
	}

	return
}

func (python Python) getOSXEnvs(original []string) []string {
	externals := []string{"readline", "openssl"}

	includeFlags := ""
	libFlags := ""

	for _, name := range externals {
		opt := "/usr/local/opt/"
		libFlags += `-L` + filepath.Join(opt, name, "lib") + " "
		includeFlags += "-I" + filepath.Join(opt, name, "include") + " "
	}

	// For zlib
	output, _ := exec.Command("xcrun", "--show-sdk-path").CombinedOutput()
	out := strings.TrimSpace(string(output))
	includeFlags += " -I" + filepath.Join(out, "/usr/include")

	original = append(original, "CPPFLAGS="+includeFlags)
	original = append(original, "LDFLAGS="+libFlags)

	return original
}

func (python Python) getCmd(args ...string) (err error, cmd *exec.Cmd, stderr, stdout io.ReadCloser) {
	cmd = exec.Command(args[0], args[1:]...)

	cmd.Env = append(os.Environ(), "LC_ALL=C") // Required for some reason
	cmd.Dir = variables.Path("python", python.Version)

	if variables.IsDebug() {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		return
	}

	stderr, err = cmd.StderrPipe()
	if err != nil {
		err = errors.New(err)
		return
	}

	// In order to preserve colors output -
	// trick the command into thinking this is real tty
	stdout, tty, err := pty.Open()
	if err != nil {
		err = errors.New(err)
		return
	}
	cmd.Stdout = tty

	return
}

func (python Python) externals() (err error) {
	path := variables.Path("python", python.Version)

	// Don't need to do anything if we already have pip and setuptools
	if hasTools(python.Version) {
		return
	}

	// Now try the "hard" way
	err = python.downloadExternals()
	if err != nil {
		return errors.New(err)
	}

	err = patch.Apply(path)
	if err != nil {
		return errors.New(err)
	}

	return
}

// Since python 3.x versions are naming their binaries with 3 affix
func (python Python) renameLinks() (err error) {
	chosen, _ := semver.Make(python.Version)
	if chosen.Major < 3 {
		return nil
	}

	path := filepath.Join(variables.Path("python", python.Version), "bin")
	rp := regexp.MustCompile("(-?)3\\.\\w")

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.New(err)
	}

	for _, file := range files {
		name := file.Name()
		absPath := filepath.Join(path, name)

		if rp.MatchString(name) {
			pathPart := rp.ReplaceAllString(name, "")
			newPath := filepath.Join(path, pathPart)

			// Since python install creates some links with version and some without
			if _, errStat := os.Lstat(newPath); errStat == nil {
				continue
			}

			err = os.Symlink(absPath, newPath)
			if err != nil {
				return errors.New(err)
			}
		}
	}

	return nil
}

func (python Python) downloadExternals() (err error) {
	path := variables.Path("python", python.Version)

	urls, err := patch.Urls(python.Version)
	if err != nil {
		return errors.New(err)
	}

	if hasTools(python.Version) == false {
		urls = append(urls, pipUrl)
	}

	respch, err := grab.GetBatch(len(urls), path, urls...)
	if err != nil {
		return errors.New(err)
	}

	// Start a ticker to update progress every 200ms
	ticker := time.NewTicker(200 * time.Millisecond)

	// Monitor downloads
	completed := 0
	responses := make([]*grab.Response, 0)
	for completed < len(urls) {
		select {
		case resp := <-respch:

			// When done
			if resp != nil {
				responses = append(responses, resp)
			}

		case <-ticker.C:

			// Update completed downloads
			for i, resp := range responses {
				if resp != nil && resp.IsComplete() {

					if resp.Error != nil {
						err = resp.Error
						return errors.New(err)
					}

					// Mark completed
					responses[i] = nil
					completed++
				}
			}
		}
	}

	ticker.Stop()

	return
}

// Since 2.7.9 versions we can simplify pip and setuptools install
func hasTools(version string) bool {
	semverVersion, _ := semver.Make(version)

	return semverVersion.Compare(pipAvailable) != -1
}

func checkErrors(out []byte) (err error) {
	output := string(out)

	if strings.Contains(output, "Traceback") {
		err = errors.New(output)
	}

	return err
}
