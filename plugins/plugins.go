// Package plugins provides essential logic for installation of the plugins
package plugins

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/chuckpreslar/emission"
	"github.com/go-errors/errors"
	"github.com/kardianos/osext"
	"github.com/markelog/archive"
	"github.com/markelog/cprf"
	"gopkg.in/cavaliercoder/grab.v1"

	"github.com/markelog/eclectica/io"
	"github.com/markelog/eclectica/pkg"
	"github.com/markelog/eclectica/shell"
	"github.com/markelog/eclectica/variables"
	"github.com/markelog/eclectica/versions"

	// plugins
	"github.com/markelog/eclectica/plugins/elm"
	"github.com/markelog/eclectica/plugins/golang"
	"github.com/markelog/eclectica/plugins/nodejs"
	"github.com/markelog/eclectica/plugins/python"
	"github.com/markelog/eclectica/plugins/ruby"
	"github.com/markelog/eclectica/plugins/rust"
)

// Plugin essential struct
type Plugin struct {
	Version string
	Pkg     pkg.Pkg
	emitter *emission.Emitter

	name string
	info map[string]string
}

// Args is arguments struct for New() method
type Args struct {
	Language    string
	Version     string
	WithModules bool
}

var (

	// Plugins holds list of all supported plugins
	Plugins = []string{
		"node",
		"rust",
		"ruby",
		"go",
		"python",
		"elm",
	}
)

// New returns new plugin struct
func New(args *Args) *Plugin {
	plugin := &Plugin{
		name:    args.Language,
		Version: args.Version,
		emitter: emission.NewEmitter(),
	}

	switch {
	case args.Language == "node":
		plugin.Pkg = nodejs.New(&nodejs.Args{
			Version:     args.Version,
			Emitter:     plugin.emitter,
			WithModules: args.WithModules,
		})
	case args.Language == "rust":
		plugin.Pkg = rust.New(args.Version, plugin.emitter)
	case args.Language == "ruby":
		plugin.Pkg = ruby.New(args.Version, plugin.emitter)
	case args.Language == "go":
		plugin.Pkg = golang.New(args.Version, plugin.emitter)
	case args.Language == "python":
		plugin.Pkg = python.New(args.Version, plugin.emitter)
	case args.Language == "elm":
		plugin.Pkg = elm.New(args.Version, plugin.emitter)
	}

	if len(args.Version) > 0 {
		plugin.info, _ = plugin.Info()
	}

	return plugin
}

// Events returns language related event emitter
func (plugin *Plugin) Events() *emission.Emitter {
	return plugin.Pkg.Events()
}

// PreDownload executes logic before downloading of the plugin
func (plugin *Plugin) PreDownload() error {
	return plugin.Pkg.PreDownload()
}

// PreInstall executes logic before installation of the plugin
func (plugin *Plugin) PreInstall() error {
	if plugin.IsInstalled() {
		return nil
	}

	return plugin.Pkg.PreInstall()
}

// LocalInstall installs language locally - to the current pwd folder
func (plugin *Plugin) LocalInstall() (err error) {
	if plugin.Version == "" {
		return errors.New("version was not defined")
	}

	// Handle CTRL+C signal
	plugin.Interrupt()

	init := shell.New(Plugins)
	init.Check()

	err = init.Initiate()
	if err != nil {
		return
	}

	// If this is already a current version we can safely say this one is installed
	if plugin.Version == plugin.Current() {
		return nil
	}

	// If it was already installed, just switch and bail out
	if plugin.IsInstalled() {
		return plugin.finishLocal()
	}

	err = plugin.Done()
	if err != nil {
		return
	}

	err = plugin.finishLocal()
	if err != nil {
		return
	}

	// Start new shell from eclectica if needed
	// note: should be the last action
	init.Start()

	return
}

func (plugin Plugin) finishLocal() (err error) {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	var (
		version = fmt.Sprintf(".%s-version", plugin.name)
		path    = filepath.Join(pwd, version)
	)

	err = plugin.Switch()
	if err != nil {
		return
	}

	err = io.WriteFile(path, plugin.Version)
	if err != nil {
		plugin.Rollback()
		return
	}

	plugin.emitter.Emit("done")

	return
}

// Install the plugin
func (plugin *Plugin) Install() (err error) {
	err = plugin.PreInstall()
	if err != nil {
		return
	}

	if plugin.Version == "" {
		return errors.New("version was not defined")
	}

	// Handle CTRL+C signal
	plugin.Interrupt()

	init := shell.New(Plugins)
	init.Check()

	err = init.Initiate()
	if err != nil {
		return
	}

	// If this is already a current version we can safely say this one is installed
	if plugin.Version == plugin.Current() {
		init.Start()
		return nil
	}

	// If it was already installed, just switch @current link if needed
	if plugin.IsInstalled() {
		err = plugin.finishInstall()
		if err != nil {
			return
		}

		init.Start()
		return
	}

	err = plugin.Done()
	if err != nil {
		return
	}

	err = plugin.finishInstall()
	if err != nil {
		return
	}

	// Start new shell from eclectica if needed
	// note: should be the last action
	init.Start()

	return
}

func (plugin Plugin) finishInstall() (err error) {
	err = plugin.Link()
	if err != nil {
		return
	}

	err = plugin.Switch()
	if err != nil {
		return
	}

	plugin.emitter.Emit("done")

	return
}

// PostInstall executes logic after installation of the plugin
func (plugin *Plugin) PostInstall() (err error) {
	if plugin.IsInstalled() {
		return nil
	}

	err = plugin.Proxy()
	if err != nil {
		plugin.Rollback()
		return
	}

	err = plugin.Pkg.PostInstall()
	if err != nil {
		plugin.Rollback()
		return
	}

	variables.WriteVersion(plugin.name, plugin.Version)

	return
}

// Switch executes logic before switching plugin versions
func (plugin *Plugin) Switch() (err error) {
	err = plugin.Pkg.Switch()
	if err != nil {
		plugin.Rollback()
		return
	}

	return
}

// Environment returns list of the all needed envionment variables
func (plugin *Plugin) Environment() ([]string, error) {
	return plugin.Pkg.Environment()
}

// Info provides all the info needed for installation of the plugin
func (plugin *Plugin) Info() (map[string]string, error) {
	if plugin.Version == "" {
		return nil, errors.New("version was not defined")
	}

	info := plugin.Pkg.Info()
	tmpDir := variables.TempDir()

	if _, ok := info["name"]; ok == false {
		info["name"] = plugin.name
	}

	if _, ok := info["version"]; ok == false {
		info["version"] = plugin.Version
	}

	if _, ok := info["extension"]; ok == false {
		info["extension"] = "tar.gz"
	}

	if _, ok := info["unarchive-filename"]; ok == false {

		// Notice different value
		info["unarchive-filename"] = info["filename"]
	}

	if _, ok := info["destination-folder"]; ok == false {
		info["destination-folder"] = filepath.Join(variables.Home(), plugin.name, plugin.Version)
	}

	if _, ok := info["archive-folder"]; ok == false {
		info["archive-folder"] = tmpDir
	}

	if _, ok := info["archive-path"]; ok == false {
		info["archive-path"] = fmt.Sprintf("%s%s.%s", info["archive-folder"], info["filename"], info["extension"])
	}

	return info, nil
}

// Current returns current used version
func (plugin *Plugin) Current() string {
	return variables.CurrentVersion(plugin.name)
}

// Rollback places everything back as it was for this language & version
func (plugin *Plugin) Rollback() {
	path := variables.Path(plugin.name, plugin.Version)
	os.RemoveAll(path)

	plugin.Pkg.Rollback()

	// If before there was more versions installed, then we can exit right here
	versions := plugin.List()
	if len(versions) > 0 {
		plugin.emitter.Emit("done")
		return
	}

	// If before there was no versions installed, then we have also remove bin proxy
	plugin.removeProxy()

	plugin.emitter.Emit("done")
}

// Done finishes installation
func (plugin *Plugin) Done() (err error) {
	err = plugin.Pkg.Install()
	if err != nil {
		plugin.Rollback()
		return
	}

	return plugin.PostInstall()
}

// Interrupt handles interruption signals (like CTRL+C)
func (plugin *Plugin) Interrupt() {
	channel := make(chan os.Signal, 1)

	plugin.emitter.Emit("done")
	signal.Notify(channel, os.Interrupt)

	go func() {
		<-channel
		plugin.Rollback()
		os.Exit(1)
	}()
}

// Remove the plugin
func (plugin *Plugin) Remove() (err error) {
	if plugin.Version == "" {
		return errors.New("version was not defined")
	}

	var (
		home = filepath.Join(variables.Home(), plugin.name)
		base = filepath.Join(home, plugin.Version)
	)

	// Need to remove proxies if this is a current version.
	// So we wouldn't confuse the user
	if plugin.Current() == plugin.Version {
		err = plugin.removeProxy()
		if err != nil {
			return err
		}
	}

	err = os.RemoveAll(base)
	if err != nil {
		return err
	}

	// If there more installed versions of the same language,
	// do not remove base + bin proxy for this language and exit early
	versions := plugin.List()
	if len(versions) > 0 {
		return nil
	}

	err = plugin.removeProxy()
	if err != nil {
		return err
	}

	return os.RemoveAll(variables.Prefix(plugin.name))
}

// Download the plugin
func (plugin *Plugin) Download() (*grab.Response, error) {
	if plugin.Version == "" {
		return nil, errors.New("version was not defined")
	}

	// If already downloaded
	if _, err := os.Stat(plugin.info["destination-folder"]); err == nil {
		return nil, nil
	}

	response, err := grab.GetAsync(plugin.info["archive-folder"], plugin.info["url"])
	if err != nil {
		return nil, err
	}

	resp := <-response

	if resp == nil {
		return resp, errors.New(variables.ConnectionError)
	}

	if resp.HTTPResponse == nil {
		return resp, errors.New(variables.ConnectionError)
	}

	if resp.HTTPResponse.StatusCode == 404 {
		grab.NewClient().CancelRequest(resp.Request)

		return resp, errors.New("Incorrect version " + plugin.Version)
	}

	return resp, nil
}

// Extract raw files from the downloaded archive (its always an archive)
func (plugin *Plugin) Extract() error {
	if plugin.Version == "" {
		return errors.New("version was not defined")
	}

	// Create language folder with path like this – /home/user/.eclectica/versions/go
	extractionPlace, err := io.CreateDir(variables.Prefix(plugin.name))
	if err != nil {
		return err
	}

	// Just in case archive was downloaded, but not extracted
	// i.e. this issue comes up at the second run.
	// Which means we will delete folder with path like this –
	// /home/user/.eclectica/versions/go1.7.1.linux-amd64
	os.RemoveAll(filepath.Join(extractionPlace, plugin.info["filename"]))

	// Now we will extract archive from
	// path like this - 					 /tmp/go1.7.1.linux-amd64.tar.gz
	// inside folder like this -   /home/user/.eclectica/versions/go
	//
	// Which will give us path like this –
	// /home/user/.eclectica/versions/go/go1.7.1.linux-amd64
	//
	// or like this – /home/user/.eclectica/versions/go/go
	//
	// Depends under what name language devs archived their dist
	err = archive.Extract(plugin.info["archive-path"], extractionPlace)
	if err != nil {
		return err
	}

	// Now we will need get path, for example - /home/user/.eclectica/versions/go/go1.7.1.linux-amd64
	tmpPath := filepath.Join(extractionPlace, plugin.info["unarchive-filename"])

	// And get path like this, for example – /home/user/.eclectica/versions/go/1.7.1
	extractionPath := plugin.info["destination-folder"]

	// Clean up in case user extracts already extracted version.
	// That might happen if this is the second pass and in first time we errored somewhere above
	os.RemoveAll(extractionPath)

	// Then rename such tmp path to what we expected to see
	err = os.Rename(tmpPath, extractionPath)
	if err != nil {
		return err
	}

	return nil
}

// Bins returns list of the all bins included
// with the distribution of the language
func (plugin *Plugin) Bins() []string {
	return plugin.Pkg.Bins()
}

// Dots returns list of the all available filenames
// which can define versions
func (plugin *Plugin) Dots() []string {
	return plugin.Pkg.Dots()
}

// List returns list of the all available local versions
func (plugin *Plugin) List() (vers []string) {
	path := variables.Prefix(plugin.name)
	vers = io.ListVersions(path)

	return
}

// ListRemote returns list of the all available remote versions
func (plugin *Plugin) ListRemote() (map[string][]string, error) {
	vers, err := plugin.Pkg.ListRemote()

	if err != nil {
		return nil, err
	}

	return versions.Compose(vers), nil
}

// Link replaces (if needed) and sets symlink for the language
func (plugin *Plugin) Link() (err error) {
	var (
		base    = variables.Path(plugin.name, plugin.Version)
		current = variables.Path(plugin.name)
	)

	err = io.Symlink(current, base)
	if err != nil {
		return
	}

	err = plugin.Pkg.Link()
	if err != nil {
		return
	}

	return
}

// IsInstalled checks if this version was already installed
func (plugin *Plugin) IsInstalled() bool {
	return variables.IsInstalled(plugin.name, plugin.Version)
}

// Proxy installs the proxy for the language
func (plugin *Plugin) Proxy() (err error) {
	ecProxyFolder := os.Getenv("EC_PROXY_PLACE")

	if ecProxyFolder == "" {
		ecProxyFolder, err = osext.ExecutableFolder()
		if err != nil {
			return
		}
	}

	executable := filepath.Join(ecProxyFolder, "ec-proxy")

	_, err = os.Stat(executable)
	if err != nil {
		if os.IsNotExist(err) {
			err = errors.New("Can't find ec-proxy binary")
		}

		return err
	}

	bins := plugin.Bins()

	for _, bin := range bins {
		err = cprf.Copy(executable, variables.DefaultInstall)
		if err != nil {
			return
		}

		fullProxy := filepath.Join(variables.DefaultInstall, "ec-proxy")
		fullBin := filepath.Join(variables.DefaultInstall, bin)

		err = os.Rename(fullProxy, fullBin)
		if err != nil {
			return
		}
	}

	return nil
}

func (plugin *Plugin) removeProxy() (err error) {
	bins := plugin.Bins()

	for _, bin := range bins {
		proxy := filepath.Join(variables.DefaultInstall, bin)

		err = os.RemoveAll(proxy)
		if err != nil {
			return
		}
	}

	return nil
}

func (plugin *Plugin) removeSupport() (err error) {
	bins := plugin.Bins()

	for _, bin := range bins {
		proxy := filepath.Join(variables.DefaultInstall, bin)

		err = os.RemoveAll(proxy)
		if err != nil {
			return
		}
	}

	return nil
}

// SearchBin searches for the actual binary
func SearchBin(name string) string {
	bins := map[string][]string{}

	for _, language := range Plugins {
		bins[language] = New(&Args{
			Language: language,
		}).Bins()
	}

	for index := range bins {
		for _, bin := range bins[index] {
			if name == bin {
				return index
			}
		}
	}

	return ""
}
