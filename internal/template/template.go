package template

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type stderrBuf struct{ bytes.Buffer }

const (
	DefaultRepo = "https://github.com/sheyaln/sabokit"
	DefaultTag  = "v0.1.0"
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
		opts.Tag = DefaultTag
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
