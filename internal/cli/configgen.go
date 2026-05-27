package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configInputs is the union of fields that land in .sabokit/config.yml.
// Empty fields are treated as "ask the user" in interactive mode and as
// "use the default or error" otherwise.
type configInputs struct {
	project    string
	baseDomain string
	defaultEnv string
	region     string
	zone       string
	sshUser    string
	sshKey     string
}

func defaultConfigInputs() configInputs {
	return configInputs{
		region:  "fr-par",
		sshUser: "ubuntu",
		sshKey:  "~/.ssh/id_ed25519",
	}
}

// promptConfigInputs fills in missing fields. interactive=true prompts the
// user for each empty field; otherwise the fields must already be populated
// or have defaults — required fields without values cause an error.
func promptConfigInputs(in *configInputs, interactive bool) error {
	if interactive {
		r := bufio.NewReader(os.Stdin)
		if in.project == "" {
			cwd, _ := os.Getwd()
			in.project = prompt(r, fmt.Sprintf("project name [%s]: ", filepath.Base(cwd)), filepath.Base(cwd))
		}
		if in.baseDomain == "" {
			in.baseDomain = prompt(r, "base domain (eg. example.com): ", "")
		}
		if in.defaultEnv == "" {
			in.defaultEnv = prompt(r, "default env (optional, eg. prod): ", "")
		}
		in.region = prompt(r, fmt.Sprintf("scaleway region [%s]: ", in.region), in.region)
		zoneDefault := in.zone
		if zoneDefault == "" {
			zoneDefault = in.region + "-1"
		}
		in.zone = prompt(r, fmt.Sprintf("scaleway zone [%s]: ", zoneDefault), zoneDefault)
		in.sshUser = prompt(r, fmt.Sprintf("ssh user [%s]: ", in.sshUser), in.sshUser)
		in.sshKey = prompt(r, fmt.Sprintf("ssh key path [%s]: ", in.sshKey), in.sshKey)
	}
	if in.project == "" {
		return fmt.Errorf("project is required")
	}
	if in.baseDomain == "" {
		return fmt.Errorf("base_domain is required")
	}
	if in.region == "" {
		in.region = "fr-par"
	}
	if in.zone == "" {
		in.zone = in.region + "-1"
	}
	if in.sshUser == "" {
		in.sshUser = "ubuntu"
	}
	if in.sshKey == "" {
		in.sshKey = "~/.ssh/id_ed25519"
	}
	return nil
}

func prompt(r *bufio.Reader, label, fallback string) string {
	fmt.Print(label)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback
	}
	return line
}

func renderConfigYAML(in configInputs) string {
	var b strings.Builder
	fmt.Fprintf(&b, "project: %s\n", in.project)
	fmt.Fprintf(&b, "base_domain: %s\n", in.baseDomain)
	if in.defaultEnv != "" {
		fmt.Fprintf(&b, "default_env: %s\n", in.defaultEnv)
	}
	fmt.Fprintf(&b, "scaleway:\n  region: %s\n  zone: %s\n", in.region, in.zone)
	fmt.Fprintf(&b, "ssh:\n  user: %s\n  key: %s\n", in.sshUser, in.sshKey)
	return b.String()
}

// writeConfigYAML writes .sabokit/config.yml under targetDir. Returns the
// path written.
func writeConfigYAML(targetDir string, in configInputs) (string, error) {
	dir := filepath.Join(targetDir, ".sabokit")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.yml")
	return path, os.WriteFile(path, []byte(renderConfigYAML(in)), 0o644)
}
