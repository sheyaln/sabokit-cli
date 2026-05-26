package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTreeSkipsGit(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	mustMkdir(t, filepath.Join(src, ".git"))
	mustMkdir(t, filepath.Join(src, "sub"))
	mustWrite(t, filepath.Join(src, "root.txt"), "root")
	mustWrite(t, filepath.Join(src, ".git", "config"), "should be skipped")
	mustWrite(t, filepath.Join(src, "sub", "nested.txt"), "nested")
	mustWrite(t, filepath.Join(src, "sub", "exec.sh"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(src, "sub", "exec.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := CopyTree(src, dst); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Errorf(".git was copied (should be skipped)")
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "root.txt")); string(b) != "root" {
		t.Errorf("root.txt content wrong: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "nested.txt")); string(b) != "nested" {
		t.Errorf("nested.txt content wrong: %q", b)
	}
	fi, err := os.Stat(filepath.Join(dst, "sub", "exec.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("exec.sh mode = %v, want 0755", fi.Mode().Perm())
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, c string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
}
