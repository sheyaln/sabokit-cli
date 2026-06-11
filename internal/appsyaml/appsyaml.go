// Package appsyaml edits environments/<env>/application.yml textually —
// uncommenting template blocks and flipping enabled flags — so the file's
// comments and ordering survive untouched. Never round-trips through a YAML
// marshaller.
package appsyaml

import (
	"fmt"
	"regexp"
	"strings"
)

// AddApp enables the named app:
//   - a commented template block (`# <name>:` + `#   key: ...` lines) is
//     uncommented (and enabled forced true);
//   - an existing block with `enabled: false` is flipped to true;
//   - an existing block without an enabled key gets `enabled: true` inserted;
//   - an absent app gets a minimal block appended (hostname FIXME'd against
//     baseDomain).
//
// Errors if the app is already enabled.
func AddApp(content, name, baseDomain string) (string, error) {
	lines := strings.Split(content, "\n")

	if start, end, ok := findBlock(lines, name); ok {
		idx, val := enabledLine(lines, start, end)
		switch {
		case idx >= 0 && val == "true":
			return "", fmt.Errorf("%s is already enabled", name)
		case idx >= 0:
			lines[idx] = "  enabled: true"
		default:
			lines = insert(lines, start+1, "  enabled: true")
		}
		return strings.Join(lines, "\n"), nil
	}

	if start, end, ok := findCommentedBlock(lines, name); ok {
		for i := start; i < end; i++ {
			lines[i] = uncomment(lines[i])
		}
		// Re-find as a live block to force enabled: true.
		out := strings.Join(lines, "\n")
		lines = strings.Split(out, "\n")
		if s, e, ok := findBlock(lines, name); ok {
			idx, val := enabledLine(lines, s, e)
			switch {
			case idx >= 0 && val != "true":
				lines[idx] = "  enabled: true"
			case idx < 0:
				lines = insert(lines, s+1, "  enabled: true")
			}
		}
		return strings.Join(lines, "\n"), nil
	}

	hostname := "FIXME." + baseDomain
	if baseDomain == "" {
		hostname = "FIXME.example.org"
	}
	block := fmt.Sprintf("\n%s:\n  enabled: true\n  hostname: %s # FIXME: set the real hostname\n", name, hostname)
	return strings.TrimRight(content, "\n") + "\n" + block, nil
}

// RemoveApp disables the named app by setting enabled: false in its block
// (inserting the line when absent). Errors if the app has no live block or
// is already disabled.
func RemoveApp(content, name string) (string, error) {
	lines := strings.Split(content, "\n")
	start, end, ok := findBlock(lines, name)
	if !ok {
		return "", fmt.Errorf("%s has no block in application.yml (already absent = already disabled)", name)
	}
	idx, val := enabledLine(lines, start, end)
	switch {
	case idx >= 0 && val == "false":
		return "", fmt.Errorf("%s is already disabled", name)
	case idx >= 0:
		lines[idx] = "  enabled: false"
	default:
		lines = insert(lines, start+1, "  enabled: false")
	}
	return strings.Join(lines, "\n"), nil
}

// findBlock locates a live top-level block: the `<name>:` line (start) and
// the line after its last member (end, exclusive).
func findBlock(lines []string, name string) (int, int, bool) {
	keyRe := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `:\s*(#.*)?$`)
	for i, l := range lines {
		if !keyRe.MatchString(l) {
			continue
		}
		end := i + 1
		for end < len(lines) {
			l := lines[end]
			// Members are indented; blanks and comments inside the block ride
			// along until the next top-level key.
			if strings.HasPrefix(l, " ") || strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
				if isTopLevelComment(l) {
					break
				}
				end++
				continue
			}
			break
		}
		return i, end, true
	}
	return 0, 0, false
}

// findCommentedBlock locates a commented-out template block: `# <name>:`
// followed by `#   key: ...` member lines.
func findCommentedBlock(lines []string, name string) (int, int, bool) {
	keyRe := regexp.MustCompile(`^#\s*` + regexp.QuoteMeta(name) + `:\s*$`)
	memberRe := regexp.MustCompile(`^#\s{2,}\S`)
	for i, l := range lines {
		if !keyRe.MatchString(l) {
			continue
		}
		end := i + 1
		for end < len(lines) && memberRe.MatchString(lines[end]) {
			end++
		}
		return i, end, true
	}
	return 0, 0, false
}

// isTopLevelComment reports whether a comment line introduces a new
// commented top-level key (`# <word>:`) rather than block-internal prose.
func isTopLevelComment(l string) bool {
	return regexp.MustCompile(`^#\s*[A-Za-z0-9_-]+:\s*$`).MatchString(l)
}

// enabledLine finds the `enabled:` member inside [start+1, end) and returns
// its index and value ("" when absent).
func enabledLine(lines []string, start, end int) (int, string) {
	re := regexp.MustCompile(`^\s+enabled:\s*(\S+)`)
	for i := start + 1; i < end && i < len(lines); i++ {
		if m := re.FindStringSubmatch(lines[i]); m != nil {
			return i, m[1]
		}
	}
	return -1, ""
}

func uncomment(l string) string {
	out := strings.TrimPrefix(l, "# ")
	if out == l {
		out = strings.TrimPrefix(l, "#")
	}
	return out
}

func insert(lines []string, at int, line string) []string {
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, line)
	out = append(out, lines[at:]...)
	return out
}
