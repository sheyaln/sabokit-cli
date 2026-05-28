package template

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type stderrBuf struct{ bytes.Buffer }

const (
	DefaultRepo = "https://github.com/sheyaln/sabokit"
	subdir      = "consumer-template"
)

type FetchOptions struct {
	Repo string
	Tag  string
}

// Fetch clones <repo> at <tag> (shallow, single-branch) into a tmpdir and
// returns the absolute path to the consumer-template subdirectory inside.
// Caller is responsible for cleaning up the parent tmpdir via CleanupParent.
func Fetch(opts FetchOptions) (string, error) {
	if opts.Repo == "" {
		opts.Repo = DefaultRepo
	}
	if opts.Tag == "" {
		opts.Tag = "master"
	}
	tmp, err := os.MkdirTemp("", "sabokit-init-*")
	if err != nil {
		return "", fmt.Errorf("create tmpdir: %w", err)
	}
	dest := filepath.Join(tmp, "src")
	c := exec.Command("git",
		"-c", "advice.detachedHead=false",
		"clone", "--depth", "1", "--branch", opts.Tag, "--quiet",
		opts.Repo, dest,
	)
	var stderr stderrBuf
	c.Stdout = io.Discard
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("git clone %s @ %s: %w\n%s", opts.Repo, opts.Tag, err, stderr.String())
	}
	src := filepath.Join(dest, subdir)
	if _, err := os.Stat(src); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("no %s/ in %s @ %s", subdir, opts.Repo, opts.Tag)
	}
	return src, nil
}

// LatestRef returns the newest blueprint tag in the given major.minor line
// (eg. "0.1") from repo's remote tags. Stable (vX.Y.Z) wins; if the line has
// only prereleases (vX.Y.Z-betaN), the newest prerelease is returned. Returns
// "" with no error when the line has no tags yet (fresh ecosystem).
func LatestRef(repo, line string) (string, error) {
	if repo == "" {
		repo = DefaultRepo
	}
	c := exec.Command("git", "ls-remote", "--tags", "--refs", repo)
	var out bytes.Buffer
	var stderr stderrBuf
	c.Stdout = &out
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		return "", fmt.Errorf("git ls-remote %s: %w\n%s", repo, err, stderr.String())
	}
	stableRe := regexp.MustCompile(`refs/tags/v` + regexp.QuoteMeta(line) + `\.(\d+)$`)
	preRe := regexp.MustCompile(`refs/tags/v` + regexp.QuoteMeta(line) + `\.(\d+)-([0-9A-Za-z.]+)$`)
	stable, stableP := "", -1
	pre, preP, preS := "", -1, ""
	for _, ln := range strings.Split(out.String(), "\n") {
		if m := stableRe.FindStringSubmatch(ln); m != nil {
			if p, _ := strconv.Atoi(m[1]); p > stableP {
				stableP, stable = p, "v"+line+"."+m[1]
			}
			continue
		}
		if m := preRe.FindStringSubmatch(ln); m != nil {
			p, _ := strconv.Atoi(m[1])
			if p > preP || (p == preP && m[2] > preS) {
				preP, preS, pre = p, m[2], "v"+line+"."+m[1]+"-"+m[2]
			}
		}
	}
	if stable != "" {
		return stable, nil
	}
	return pre, nil
}

// CleanupParent removes the tmpdir parent of a path returned by Fetch.
func CleanupParent(srcPath string) {
	parent := filepath.Dir(filepath.Dir(srcPath))
	if parent != "" && parent != "/" {
		os.RemoveAll(parent)
	}
}

// CopyTree copies everything under src into dst, preserving file modes.
// Skips .git directories.
func CopyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
