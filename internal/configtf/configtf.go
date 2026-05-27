// Package configtf provides line-based edits to consumer-template's
// environments/<env>/config.tf. The shape we care about:
//
//	locals {
//	  config = {
//	    apps = {
//	      outline = {
//	        enabled  = true
//	        hostname = "wiki.example.org"
//	      }
//
//	      # vikunja = {
//	      #   enabled  = true
//	      #   hostname = "tasks.example.org"
//	      # }
//	    }
//	  }
//	}
//
// The package is line-based regex editing — HCL AST round-trips would lose
// formatting and comments. Indentation discovery is done per-call so the
// edits work regardless of project-specific formatting.
package configtf

import (
	"fmt"
	"regexp"
	"strings"
)

// AppStatus reports the current state of <name> within the config.tf apps
// block.
type AppStatus int

const (
	Absent       AppStatus = iota // no entry, commented or otherwise
	CommentedOut                  // entry exists but every line is # prefixed
	Enabled                       // active block with enabled = true (or no explicit enabled)
	Disabled                      // active block with enabled = false
)

// FindApp scans content for an entry named <name> in the apps block.
func FindApp(content, name string) (AppStatus, error) {
	if name == "" {
		return Absent, fmt.Errorf("app name is empty")
	}
	lines := strings.Split(content, "\n")
	idx, kind := findAppBlock(lines, name)
	if idx == -1 {
		return Absent, nil
	}
	if kind == "commented" {
		return CommentedOut, nil
	}
	// active block — scan its body for `enabled = false` or `enabled = true`
	endIdx := findBlockEnd(lines, idx, false)
	if endIdx == -1 {
		return Enabled, nil
	}
	enabledRe := regexp.MustCompile(`^\s*enabled\s*=\s*(true|false)\b`)
	for i := idx + 1; i < endIdx; i++ {
		m := enabledRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		if m[1] == "false" {
			return Disabled, nil
		}
		return Enabled, nil
	}
	return Enabled, nil
}

// AddApp enables <name> in config.tf. If there's a commented-out block, it
// is uncommented. Otherwise a minimal block is inserted at the end of the
// apps = { ... } container.
func AddApp(content, name string) (string, error) {
	status, err := FindApp(content, name)
	if err != nil {
		return "", err
	}
	switch status {
	case Enabled:
		return "", fmt.Errorf("%s is already enabled", name)
	case Disabled:
		// flip the enabled = false → true
		return setEnabled(content, name, true)
	case CommentedOut:
		return uncommentAppBlock(content, name)
	case Absent:
		return insertAppBlock(content, name)
	}
	return "", fmt.Errorf("unreachable: %s status %d", name, status)
}

// RemoveApp disables <name> in config.tf — sets enabled = false within
// the named block. Inserts the line if missing. Errors if no block exists.
func RemoveApp(content, name string) (string, error) {
	status, err := FindApp(content, name)
	if err != nil {
		return "", err
	}
	switch status {
	case Absent, CommentedOut:
		return "", fmt.Errorf("%s is not enabled (nothing to remove)", name)
	case Disabled:
		return "", fmt.Errorf("%s is already disabled", name)
	}
	return setEnabled(content, name, false)
}

// findAppBlock returns the line index of the `<name> = {` line and a kind
// string ("active" or "commented"), or -1 if not found.
func findAppBlock(lines []string, name string) (int, string) {
	commented := regexp.MustCompile(`^\s*#\s*` + regexp.QuoteMeta(name) + `\s*=\s*\{`)
	active := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(name) + `\s*=\s*\{`)
	for i, line := range lines {
		if active.MatchString(line) && !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			return i, "active"
		}
		if commented.MatchString(line) {
			return i, "commented"
		}
	}
	return -1, ""
}

// findBlockEnd returns the line index of the matching `}` line. The block
// can be either commented (every line prefixed with #) or active.
func findBlockEnd(lines []string, startIdx int, commented bool) int {
	open := 1
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if commented {
			line = stripCommentPrefix(line)
		}
		// count { and } on this line
		open += strings.Count(line, "{")
		open -= strings.Count(line, "}")
		if open <= 0 {
			return i
		}
	}
	return -1
}

func stripCommentPrefix(line string) string {
	idx := strings.Index(line, "#")
	if idx < 0 {
		return line
	}
	rest := line[idx+1:]
	if strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}
	return line[:idx] + rest
}

func uncommentAppBlock(content, name string) (string, error) {
	lines := strings.Split(content, "\n")
	startIdx, kind := findAppBlock(lines, name)
	if startIdx == -1 || kind != "commented" {
		return "", fmt.Errorf("no commented-out block for %s", name)
	}
	endIdx := findBlockEnd(lines, startIdx, true)
	if endIdx == -1 {
		return "", fmt.Errorf("unmatched commented block for %s starting at line %d", name, startIdx+1)
	}
	for i := startIdx; i <= endIdx; i++ {
		lines[i] = stripCommentPrefix(lines[i])
	}
	return strings.Join(lines, "\n"), nil
}

func setEnabled(content, name string, enabled bool) (string, error) {
	lines := strings.Split(content, "\n")
	startIdx, kind := findAppBlock(lines, name)
	if startIdx == -1 || kind != "active" {
		return "", fmt.Errorf("no active block for %s", name)
	}
	endIdx := findBlockEnd(lines, startIdx, false)
	if endIdx == -1 {
		return "", fmt.Errorf("unmatched block for %s", name)
	}
	enabledRe := regexp.MustCompile(`^(\s*enabled\s*=\s*)(true|false)(\b.*)$`)
	target := "false"
	if enabled {
		target = "true"
	}
	for i := startIdx + 1; i < endIdx; i++ {
		if m := enabledRe.FindStringSubmatch(lines[i]); m != nil {
			lines[i] = m[1] + target + m[3]
			return strings.Join(lines, "\n"), nil
		}
	}
	// no enabled line; insert one right after the opening brace
	indent := detectInnerIndent(lines, startIdx, endIdx)
	insert := indent + "enabled = " + target
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:startIdx+1]...)
	out = append(out, insert)
	out = append(out, lines[startIdx+1:]...)
	return strings.Join(out, "\n"), nil
}

func detectInnerIndent(lines []string, start, end int) string {
	leading := regexp.MustCompile(`^([ \t]+)`)
	for i := start + 1; i < end; i++ {
		if m := leading.FindStringSubmatch(lines[i]); m != nil {
			return m[1]
		}
	}
	// fall back to outer indent + 2 spaces
	if m := leading.FindStringSubmatch(lines[start]); m != nil {
		return m[1] + "  "
	}
	return "  "
}

func insertAppBlock(content, name string) (string, error) {
	lines := strings.Split(content, "\n")
	// find the apps = { line that starts the apps container, then walk to
	// its matching } and insert just before it.
	appsStart := -1
	appsRe := regexp.MustCompile(`^\s*apps\s*=\s*\{`)
	for i, line := range lines {
		if appsRe.MatchString(line) && !strings.HasPrefix(strings.TrimLeft(line, " \t"), "#") {
			appsStart = i
			break
		}
	}
	if appsStart == -1 {
		return "", fmt.Errorf("no apps = { block found in config.tf — add the block manually first")
	}
	appsEnd := findBlockEnd(lines, appsStart, false)
	if appsEnd == -1 {
		return "", fmt.Errorf("unmatched apps block")
	}
	indent := detectInnerIndent(lines, appsStart, appsEnd)
	innerIndent := indent + "  "
	block := []string{
		"",
		indent + name + " = {",
		innerIndent + "enabled  = true",
		innerIndent + `hostname = ""  # FIXME: set hostname`,
		indent + "}",
	}
	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:appsEnd]...)
	out = append(out, block...)
	out = append(out, lines[appsEnd:]...)
	return strings.Join(out, "\n"), nil
}
