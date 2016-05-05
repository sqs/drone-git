package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/drone/drone-plugin-go/plugin"
)

var netrcFile = `
machine %s
login %s
password %s
`

// Params stores the git clone parameters used to
// configure and customzie the git clone behavior.
type Params struct {
	Depth              int               `json:"depth"`
	Recursive          bool              `json:"recursive"`
	SkipVerify         bool              `json:"skip_verify"`
	Tags               bool              `json:"tags"`
	Submodules         map[string]string `json:"submodule_override"`
	SubmoduleRemote    bool              `json:"submodule_update_remote"`
	TweakGitattributes bool              `json:"tweak_gitattributes"`
}

func main() {
	v := new(Params)
	r := new(plugin.Repo)
	b := new(plugin.Build)
	w := new(plugin.Workspace)
	plugin.Param("repo", r)
	plugin.Param("build", b)
	plugin.Param("workspace", w)
	plugin.Param("vargs", &v)
	plugin.MustParse()

	err := clone(r, b, w, v)
	if err != nil {
		os.Exit(1)
	}
}

// Clone clones the repository and build revision
// into the build workspace.
func clone(r *plugin.Repo, b *plugin.Build, w *plugin.Workspace, v *Params) error {
	if v.Depth == 0 {
		v.Depth = 50
	}

	err := os.MkdirAll(w.Path, 0777)
	if err != nil {
		fmt.Printf("Error creating directory %s. %s\n", w.Path, err)
		return err
	}

	// generate the .netrc file
	if err := writeNetrc(w); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	// write the rsa private key if provided
	if err := writeKey(w); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}

	var cmds []*exec.Cmd

	if v.SkipVerify {
		cmds = append(cmds, skipVerify())
	}

	// check for a .git directory and whether it's empty
	if isDirEmpty(filepath.Join(w.Path, ".git")) {
		cmds = append(cmds, initGit())
		cmds = append(cmds, remote(r))
	}

	switch {
	case isPullRequest(b) || isTag(b):
		cmds = append(cmds, fetch(b, v.Tags, v.Depth))
		cmds = append(cmds, checkoutHead(b))
	default:
		cmds = append(cmds, fetch(b, v.Tags, v.Depth))
		cmds = append(cmds, checkoutSha(b))
	}

	for name, url := range v.Submodules {
		cmds = append(cmds, remapSubmodule(name, url))
	}

	if v.Recursive {
		cmds = append(cmds, updateSubmodules(v.SubmoduleRemote))
	}

	execute(cmds, w.Path)

	if v.TweakGitattributes {
		tweakGitattributes(w.Path)
	}

	return nil
}

// Creates an empty git repository.
func initGit() *exec.Cmd {
	return exec.Command(
		"git",
		"init",
	)
}

// Sets the remote origin for the repository.
func remote(r *plugin.Repo) *exec.Cmd {
	return exec.Command(
		"git",
		"remote",
		"add",
		"origin",
		r.Clone,
	)
}

// Checkout executes a git checkout command.
func checkoutHead(b *plugin.Build) *exec.Cmd {
	return exec.Command(
		"git",
		"checkout",
		"-qf",
		"FETCH_HEAD",
	)
}

// Checkout executes a git checkout command.
func checkoutSha(b *plugin.Build) *exec.Cmd {
	return exec.Command(
		"git",
		"reset",
		"--hard",
		"-q",
		b.Commit,
	)
}

// fetch retuns git command that fetches from origin. If tags is true
// then tags will be fetched.
func fetch(b *plugin.Build, tags bool, depth int) *exec.Cmd {
	tags_option := "--no-tags"
	if tags {
		tags_option = "--tags"
	}
	return exec.Command(
		"git",
		"fetch",
		tags_option,
		fmt.Sprintf("--depth=%d", depth),
		"origin",
		fmt.Sprintf("+%s:", b.Ref),
	)
}

// updateSubmodules recursively initializes and updates submodules.
func updateSubmodules(remote bool) *exec.Cmd {
	cmd := exec.Command(
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
	)

	if remote {
		cmd.Args = append(cmd.Args, "--remote")
	}

	return cmd
}

// skipVerify returns a git command that, when executed
// configures git to skip ssl verification. This should
// may be used with self-signed certificates.
func skipVerify() *exec.Cmd {
	return exec.Command(
		"git",
		"config",
		"--global",
		"http.sslVerify",
		"false",
	)
}

// remapSubmodule returns a git command that, when executed
// configures git to remap submodule urls.
func remapSubmodule(name, url string) *exec.Cmd {
	name = fmt.Sprintf("submodule.%s.url", name)
	return exec.Command(
		"git",
		"config",
		name,
		url,
	)
}

// Trace writes each command to standard error (preceded by a ‘$ ’) before it
// is executed. Used for debugging your build.
func trace(cmd *exec.Cmd) {
	fmt.Println("$", strings.Join(cmd.Args, " "))
}

// Writes the netrc file.
func writeNetrc(in *plugin.Workspace) error {
	if in.Netrc == nil || len(in.Netrc.Machine) == 0 {
		return nil
	}
	out := fmt.Sprintf(
		netrcFile,
		in.Netrc.Machine,
		in.Netrc.Login,
		in.Netrc.Password,
	)
	home := "/root"
	u, err := user.Current()
	if err == nil {
		home = u.HomeDir
	}
	path := filepath.Join(home, ".netrc")
	return ioutil.WriteFile(path, []byte(out), 0600)
}

// Writes the RSA private key
func writeKey(in *plugin.Workspace) error {
	if in.Keys == nil || len(in.Keys.Private) == 0 {
		return nil
	}
	home := "/root"
	u, err := user.Current()
	if err == nil {
		home = u.HomeDir
	}
	sshpath := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshpath, 0700); err != nil {
		return err
	}
	confpath := filepath.Join(sshpath, "config")
	privpath := filepath.Join(sshpath, "id_rsa")
	ioutil.WriteFile(confpath, []byte("StrictHostKeyChecking no\n"), 0700)
	return ioutil.WriteFile(privpath, []byte(in.Keys.Private), 0600)
}

func isDirEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return true
	}
	defer f.Close()

	_, err = f.Readdir(1)
	if err == io.EOF {
		return true
	}
	return false
}

func isPullRequest(b *plugin.Build) bool {
	return b.Event == plugin.EventPull
}

func isTag(b *plugin.Build) bool {
	return b.Event == plugin.EventTag ||
		strings.HasPrefix(b.Ref, "refs/tags/")
}

// execute executes all the commands given using specified directory as CWD
func execute(cmds []*exec.Cmd, cwd string) error {
	for _, cmd := range cmds {
		cmd.Dir = cwd
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		trace(cmd)
		err := cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

// Describes .gitattribute tweaking rules in form
// RE pattern => replacement string (literal)
type tweak struct {
	pattern     *regexp.Regexp
	replacement string
}

// .gitattribute tweaking rules
var tweaks = make([]tweak, 0, 3)

// initializes .gitattribute tweaking rules
// current rules are:
// - transform "eol=crlf" to "eol=lf" (ensure that LF line endings are used regardless of OS and repository configuration)
// - remove "ident" attributes to prevent $Id$ substitution
// - remove filter=NAME to disable filters invocation
func init() {
	p, err := regexp.Compile(`\beol=crlf\b`)
	if err != nil {
		panic(err)
	}
	tweaks = append(tweaks, tweak{p, "eol=lf"})
	p, err = regexp.Compile(`([^=]|^)ident\b`)
	if err != nil {
		panic(err)
	}
	tweaks = append(tweaks, tweak{p, ""})
	p, err = regexp.Compile(`\bfilter=(\S+)`)
	if err != nil {
		panic(err)
	}
	tweaks = append(tweaks, tweak{p, ""})
}

// tweakGitattributes tweaks .gitattributes in the given directory
func tweakGitattributes(directory string) error {
	attrsFile := filepath.Join(directory, ".gitattributes")
	attrs, err := ioutil.ReadFile(attrsFile)
	// don't care if we were unable to read .gitattributes
	if err != nil {
		return nil
	}

	tweaked := false
	for _, t := range tweaks {
		if !t.pattern.Match(attrs) {
			continue
		}
		attrs = t.pattern.ReplaceAllLiteral(attrs, []byte(t.replacement))
		tweaked = true
	}

	if !tweaked {
		return nil
	}
	err = ioutil.WriteFile(attrsFile, attrs, 0600)
	if err != nil {
		return err
	}

	cmds := []*exec.Cmd{
		// add updated .gitattributes
		exec.Command("git", "add", ".gitattributes"),
		// commit updated .gitattributes
		exec.Command("git", "commit", "-m", "tweaking .gitattributes"),
		// remove cloned tree's files
		exec.Command("git", "rm", "--cached", "-r", "."),
		// reset cloned repository in order to take into account tweaked .gitattributes
		exec.Command("git", "reset", "--hard"),
		// remove commit of .gitattributes
		exec.Command("git", "reset", "HEAD~"),
		// restore original .gitattributes locally
		exec.Command("git", "checkout", ".gitattributes"),
	}
	return execute(cmds, directory)
}
