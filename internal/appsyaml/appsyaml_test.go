package appsyaml

import (
	"strings"
	"testing"
)

const sample = `# Application-layer config.

outline:
  enabled: true
  hostname: wiki.example.org
  # authorized_groups: [member]

# vikunja:
#   enabled: true
#   hostname: tasks.example.org

espocrm:
  enabled: false
  hostname: crm.example.org

n8n:
  hostname: automate.example.org
`

func TestAddUncommentsTemplateBlock(t *testing.T) {
	out, err := AddApp(sample, "vikunja", "example.org")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "vikunja:\n  enabled: true\n  hostname: tasks.example.org") {
		t.Errorf("commented block not uncommented:\n%s", out)
	}
	if strings.Contains(out, "# vikunja:") {
		t.Errorf("commented key survived:\n%s", out)
	}
}

func TestAddFlipsDisabled(t *testing.T) {
	out, err := AddApp(sample, "espocrm", "example.org")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "espocrm:\n  enabled: true") {
		t.Errorf("enabled not flipped:\n%s", out)
	}
}

func TestAddInsertsEnabledWhenMissing(t *testing.T) {
	out, err := AddApp(sample, "n8n", "example.org")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "n8n:\n  enabled: true\n  hostname: automate.example.org") {
		t.Errorf("enabled not inserted:\n%s", out)
	}
}

func TestAddAlreadyEnabled(t *testing.T) {
	if _, err := AddApp(sample, "outline", "example.org"); err == nil {
		t.Fatal("expected already-enabled error")
	}
}

func TestAddAppendsMissing(t *testing.T) {
	out, err := AddApp(sample, "jitsi", "example.org")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "jitsi:\n  enabled: true\n  hostname: FIXME.example.org") {
		t.Errorf("minimal block not appended:\n%s", out)
	}
}

func TestRemoveFlipsEnabled(t *testing.T) {
	out, err := RemoveApp(sample, "outline")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "outline:\n  enabled: false") {
		t.Errorf("enabled not flipped to false:\n%s", out)
	}
	// neighbours untouched
	if !strings.Contains(out, "# vikunja:") {
		t.Errorf("unrelated commented block touched:\n%s", out)
	}
}

func TestRemoveInsertsEnabledFalse(t *testing.T) {
	out, err := RemoveApp(sample, "n8n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "n8n:\n  enabled: false\n  hostname: automate.example.org") {
		t.Errorf("enabled: false not inserted:\n%s", out)
	}
}

func TestRemoveAlreadyDisabled(t *testing.T) {
	if _, err := RemoveApp(sample, "espocrm"); err == nil {
		t.Fatal("expected already-disabled error")
	}
}

func TestRemoveAbsent(t *testing.T) {
	if _, err := RemoveApp(sample, "vikunja"); err == nil {
		t.Fatal("expected no-block error for commented-only app")
	}
}
