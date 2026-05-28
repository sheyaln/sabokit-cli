package cli

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/envvalues"
	"github.com/sheyaln/sabokit-cli/internal/inventory"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/scw"
	"github.com/sheyaln/sabokit-cli/internal/tf"
	"github.com/sheyaln/sabokit-cli/internal/wait"
	"github.com/spf13/cobra"
)

type upFlags struct {
	skipPreflight bool
	skipUp        bool
	skipConfigure bool
	backendConfig string
	parallelism   int
	noConfirm     bool
}

func newUpCmd() *cobra.Command {
	f := &upFlags{}
	cmd := &cobra.Command{
		Use:   "up",
		Short: "provision + configure the current env (pure Go; no local terraform/ansible/scw needed)",
		Long: `runs the full first-deploy chain against the env at environments/<env>/.
nothing is shelled to host-installed terraform/ansible/scw/jq/python3 —
every step calls docker images directly:

  terraform   ` + DefaultTFImage + ` (override with --tf-image or SABOKIT_TF_IMAGE)
  ansible     sabokit-runner (--image / SABOKIT_IMAGE)
  scw         scaleway/cli (--scw-image / SABOKIT_SCW_IMAGE)

phases:
  preflight:
    config.tf + backend.hcl present, required keys non-empty,
    SCW creds reach the project, ssh public key in scaleway IAM,
    base_domain's DNS zone registered with scaleway.
  up (1/8..8/8):
    1. terraform init + plan + confirm + apply (base + identity_bootstrap)
    2. refresh .tf-output.json, inventory.ini, .ansible-vars.json
    3. clear stale known_hosts entries
    4. wait for SSH on every host (port 22)
    5. ansible-playbook bootstrap.yml
    6. wait for gateway DNS propagation
    7. wait for Let's Encrypt cert on the gateway
    8. wait for Authentik blueprints + RBAC permissions to index
  configure (1/4..4/4):
    1. read Authentik admin token from scaleway secret
    2. import authentik_outpost.embedded if not already in state
    3. plan + confirm + apply (full, with -var authentik_admin_token=...)
    4. refresh .tf-output.json + inventory.ini + .ansible-vars.json

every terraform apply runs 'terraform plan -out=.tfplan' first, prints
the diff, and prompts before applying. default is yes in non-prod envs
and no in prod. --no-confirm bypasses every prompt (auto-approve every
apply; required for unattended runs).

deploy/down/status also re-run the refresh step automatically before any
ansible/tf call — inventory.ini is always derived from current TF state.
opt out with --skip-refresh on any of those commands.

requires env's config.tf, backend.hcl, inventory.ini in place. uses
SCW_ACCESS_KEY / SCW_SECRET_KEY from the environment for terraform's
Scaleway provider and scw secret manager access.`,
		Example: `  sabokit up                       # full first deploy
  sabokit up --skip-up             # just re-run configure
  sabokit up --skip-configure      # just run up phases (no config)
  sabokit --env staging up`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(f)
		},
	}
	cmd.Flags().BoolVar(&f.skipPreflight, "skip-preflight", false, "skip phase 0 (config + creds + ssh key checks)")
	cmd.Flags().BoolVar(&f.skipUp, "skip-up", false, "skip phases 1-9 (up.sh equivalent)")
	cmd.Flags().BoolVar(&f.skipConfigure, "skip-configure", false, "skip phases 10-13 (configure.sh equivalent)")
	cmd.Flags().StringVar(&f.backendConfig, "backend-config", "backend.hcl", "backend config filename inside the env dir")
	cmd.Flags().IntVar(&f.parallelism, "parallelism", 3, "terraform -parallelism for the configure-phase apply")
	cmd.Flags().BoolVar(&f.noConfirm, "no-confirm", false, "skip the plan-then-confirm gate (auto-approve every apply; required for unattended runs)")
	return cmd
}

func runUp(f *upFlags) error {
	if f.skipPreflight && f.skipUp && f.skipConfigure {
		return fmt.Errorf("--skip-preflight, --skip-up, and --skip-configure together would run nothing")
	}
	if err := docker.Preflight(); err != nil {
		return err
	}
	p, err := project.Load()
	if err != nil {
		return err
	}
	if err := requireCompatibleBlueprint(p); err != nil {
		return err
	}
	envName := p.EnvName(globals.Env)
	if envName == "" {
		return fmt.Errorf("up requires an env (pass --env or set default_env in .sabokit/config.yml)")
	}
	envDir, err := p.WorkspaceDir(globals.Env)
	if err != nil {
		return err
	}

	tfClient := tf.New(globals.TFImage, globals.Platform, p.Root)
	scwClient := scw.New(globals.ScwImage, globals.Platform)

	if !f.skipPreflight {
		if err := runPreflight(p, envName, envDir, scwClient); err != nil {
			return err
		}
	} else {
		phase("skipping preflight (--skip-preflight)")
	}

	if !f.skipUp {
		if err := runUpPhases(p, envName, envDir, tfClient, scwClient, f); err != nil {
			return err
		}
	} else {
		phase("skipping up phases (--skip-up)")
	}

	if !f.skipConfigure {
		if err := runConfigurePhases(p, envName, envDir, tfClient, scwClient, f); err != nil {
			return err
		}
	} else {
		phase("skipping configure phases (--skip-configure)")
	}

	phase("done")
	return nil
}

func phase(msg string) {
	fmt.Printf("==> %s\n", msg)
}

// planConfirmApply is the shared "show the plan, ask, apply" gate around
// every terraform apply sabokit runs. When --no-confirm is set it falls
// back to auto-approve. Otherwise: terraform plan -out=.tfplan streams
// the diff to the user, confirmPlan reads y/n from stdin, and
// terraform apply .tfplan commits the saved plan unchanged.
func planConfirmApply(tfClient *tf.Client, envDir, envName string, opts tf.ApplyOpts, noConfirm bool) error {
	if noConfirm {
		opts.AutoApprove = true
		return tfClient.Apply(envDir, opts)
	}
	if err := tfClient.Plan(envDir, opts); err != nil {
		return fmt.Errorf("terraform plan: %w", err)
	}
	ok, err := confirmPlan(envName, os.Stdin)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("aborted by user")
	}
	return tfClient.ApplyPlan(envDir)
}

// confirmPlan prompts the operator after a `terraform plan`. Default is
// yes for non-prod envs (Enter accepts), no for prod (Enter aborts).
// Reads exactly one line from in.
func confirmPlan(envName string, in io.Reader) (bool, error) {
	isProd := envName == "prod"
	if isProd {
		fmt.Print("proceed with this plan? [y/N] ")
	} else {
		fmt.Print("proceed with this plan? [Y/n] ")
	}
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return !isProd, nil
	}
	return line == "y" || line == "yes", nil
}

// runPreflight verifies the env is in a state where `up` can succeed:
// required files present, env-values.yml has this env's required keys (and no
// two envs share a project_id), SCW creds work, and the user's SSH public key
// is in the project's IAM keystore. Tools come from docker images, so there
// are no host dep-install checks.
func runPreflight(p *project.Project, envName, envDir string, scwClient *scw.Client) error {
	phase("preflight")

	required := []string{"config.tf", "backend.hcl"}
	for _, f := range required {
		if _, err := os.Stat(filepath.Join(envDir, f)); err != nil {
			return fmt.Errorf("preflight: %s/%s missing — copy from %s.example and edit", envDir, f, f)
		}
	}

	// Per-env values live in the committed, keyed environments/env-values.yml,
	// which Terraform resolves itself via env.tf (yamldecode + dir name). The
	// CLI reads the same file here — it's not a CLI-only input.
	allEnvs, err := envvalues.Load(p.Root)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if err := envvalues.CheckDistinctProjectIDs(allEnvs); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	slice, err := envvalues.Get(p.Root, envName)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if err := slice.Require(envvalues.RequiredKeys...); err != nil {
		return fmt.Errorf("preflight: env-values.yml [%s]: %w", envName, err)
	}
	projectID := slice.String("scaleway_project_id")
	baseDomain := slice.String("base_domain")
	gateway := slice.String("identity_domain")
	if !strings.HasSuffix(gateway, "."+baseDomain) && gateway != baseDomain {
		fmt.Fprintf(os.Stderr, "    warning: identity_domain %q does not sit under base_domain %q — terraform may refuse to manage the A record\n", gateway, baseDomain)
	}

	if os.Getenv("SCW_ACCESS_KEY") == "" || os.Getenv("SCW_SECRET_KEY") == "" {
		return fmt.Errorf("preflight: SCW_ACCESS_KEY and SCW_SECRET_KEY must be exported in the environment")
	}

	if err := scwClient.AccountProjectGet(projectID); err != nil {
		return fmt.Errorf("preflight: scw cannot reach project %s with the current credentials: %w", projectID, err)
	}

	if err := ensureSSHKeyUploaded(p, envName, projectID, scwClient); err != nil {
		return err
	}
	if err := ensureDNSZoneDelegated(baseDomain, scwClient); err != nil {
		return err
	}
	return nil
}

// ensureDNSZoneDelegated confirms baseDomain's apex zone is registered
// with scaleway and the zone is delegated to scaleway's nameservers.
// Returns a clear error if missing; emits a warning (no fail) if the
// zone is registered but delegated elsewhere — sabokit can still create
// records, they just won't resolve until the user moves the NS records.
func ensureDNSZoneDelegated(baseDomain string, scwClient *scw.Client) error {
	zone, err := scwClient.FindApexZone(baseDomain)
	if err != nil {
		return fmt.Errorf("preflight: scw dns zone list: %w", err)
	}
	if zone == nil {
		return fmt.Errorf("preflight: dns zone %q not registered with scaleway — add it in the scaleway console (DNS > Zones) before continuing", baseDomain)
	}
	if !scw.IsDelegatedToScaleway(zone) {
		fmt.Fprintf(os.Stderr, "    warning: zone %q registered but not delegated to scaleway (NS: %v) — set NS records at your registrar to ns0.dom.scw.cloud / ns1.dom.scw.cloud, or A records won't resolve until you do\n", baseDomain, zone.NS)
	}
	return nil
}

// ensureSSHKeyUploaded reads the user's SSH public key and ensures it's
// in the scaleway project's IAM keystore. The private key path comes from
// .sabokit/config.yml's ssh.key — we append .pub for the public side.
func ensureSSHKeyUploaded(p *project.Project, envName, projectID string, scwClient *scw.Client) error {
	privPath := p.Config.SSH.Key
	if privPath == "" {
		privPath = "~/.ssh/id_ed25519"
	}
	pubPath := expandHome(privPath) + ".pub"
	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return fmt.Errorf("preflight: read %s: %w (generate with `ssh-keygen -t ed25519` or update .sabokit/config.yml ssh.key)", pubPath, err)
	}
	host, _ := os.Hostname()
	name := fmt.Sprintf("sabokit-%s-%s", host, envName)
	if err := scwClient.EnsureSSHKey(name, string(pub), projectID); err != nil {
		return fmt.Errorf("preflight: upload ssh key to scaleway IAM: %w", err)
	}
	return nil
}

// runUpPhases runs phases 1-8: bootstrap apply (plan+confirm), refresh,
// ssh wait, ansible bootstrap, DNS propagation, LE cert wait, blueprint
// indexing wait.
func runUpPhases(p *project.Project, envName, envDir string, tfClient *tf.Client, scwClient *scw.Client, f *upFlags) error {
	phase("1/8 terraform init + plan + apply (base + identity_bootstrap)")
	if err := tfClient.Init(envDir, f.backendConfig); err != nil {
		return fmt.Errorf("terraform init: %w", err)
	}
	if err := planConfirmApply(tfClient, envDir, envName, tf.ApplyOpts{
		Targets: []string{"module.stack.module.base", "module.stack.module.identity_bootstrap"},
	}, f.noConfirm); err != nil {
		return fmt.Errorf("bootstrap apply: %w", err)
	}

	phase("2/8 refreshing .tf-output.json, inventory.ini, .ansible-vars.json")
	tfOut, err := refreshEnvState(envDir, envName, tfClient)
	if err != nil {
		return err
	}

	hostIPs, err := extractHostIPs(tfOut)
	if err != nil {
		return err
	}

	phase("3/8 clearing stale known_hosts entries")
	if err := pruneKnownHosts(hostIPs); err != nil {
		fmt.Fprintf(os.Stderr, "warning: prune known_hosts: %v\n", err)
	}

	phase(fmt.Sprintf("4/8 waiting for SSH on %d host(s)", len(hostIPs)))
	for _, ip := range hostIPs {
		fmt.Printf("    waiting for %s:22 ...\n", ip)
		if err := wait.TCP(ip+":22", wait.DefaultTCP()); err != nil {
			return err
		}
	}

	gateway, err := readGatewayDomain(p.Root, envName)
	if err != nil {
		return err
	}

	phase("5/8 ansible-playbook bootstrap.yml")
	if err := runAnsibleBootstrap(p, envName, envDir, gateway); err != nil {
		return err
	}

	phase(fmt.Sprintf("6/8 waiting for gateway DNS propagation (%s)", gateway))
	if err := wait.Resolve(gateway, wait.DefaultResolve(), nil, nil); err != nil {
		return fmt.Errorf("gateway DNS never resolved: %w (check NS delegation + A record)", err)
	}

	phase(fmt.Sprintf("7/8 waiting for Let's Encrypt cert on https://%s", gateway))
	if err := waitLECert(envName, envDir, gateway, hostIPs); err != nil {
		return err
	}

	phase("8/8 waiting for Authentik blueprints + RBAC to index")
	if err := waitAuthentikIndexing(scwClient, gateway, tfOut); err != nil {
		return err
	}
	return nil
}

// runConfigurePhases runs phases 10-13: read admin token, optional outpost
// import, full terraform apply, refresh outputs.
func runConfigurePhases(p *project.Project, envName, envDir string, tfClient *tf.Client, scwClient *scw.Client, f *upFlags) error {
	tfOutPath := filepath.Join(envDir, ".tf-output.json")
	tfOut, err := os.ReadFile(tfOutPath)
	if err != nil {
		return fmt.Errorf("read %s: %w (run sabokit up without --skip-up first)", tfOutPath, err)
	}

	phase("configure 1/4 reading Authentik admin token from Scaleway")
	adminToken, err := readAuthentikAdminToken(scwClient, tfOut)
	if err != nil {
		return err
	}

	gateway, err := readGatewayDomain(p.Root, envName)
	if err != nil {
		return err
	}

	phase("configure 2/4 reconciling Authentik embedded outpost in TF state")
	if err := reconcileOutpost(tfClient, envDir, gateway, adminToken); err != nil {
		return err
	}

	phase(fmt.Sprintf("configure 3/4 terraform plan + apply (full, parallelism=%d)", f.parallelism))
	if err := planConfirmApply(tfClient, envDir, envName, tf.ApplyOpts{
		Parallelism: f.parallelism,
		Vars:        map[string]string{"authentik_admin_token": adminToken},
	}, f.noConfirm); err != nil {
		return fmt.Errorf("configure apply: %w", err)
	}

	phase("configure 4/4 refreshing .tf-output.json + inventory.ini + .ansible-vars.json")
	if _, err := refreshEnvState(envDir, envName, tfClient); err != nil {
		return err
	}
	return nil
}

// extractHostIPs pulls the public_ip out of every entry in compute_hosts.
func extractHostIPs(tfOut []byte) ([]string, error) {
	var doc map[string]struct {
		Value map[string]inventory.ComputeHost `json:"value"`
	}
	if err := json.Unmarshal(tfOut, &doc); err != nil {
		return nil, err
	}
	hosts, ok := doc["compute_hosts"]
	if !ok {
		return nil, fmt.Errorf("compute_hosts not in TF output")
	}
	ips := make([]string, 0, len(hosts.Value))
	for _, h := range hosts.Value {
		if h.PublicIP != "" {
			ips = append(ips, h.PublicIP)
		}
	}
	return ips, nil
}

// pruneKnownHosts removes lines from ~/.ssh/known_hosts that begin with
// any of the given IPs. Avoids the host-key-changed prompt after a
// re-provision swaps an IP's machine.
func pruneKnownHosts(ips []string) error {
	if len(ips) == 0 {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	ipSet := map[string]bool{}
	for _, ip := range ips {
		ipSet[ip] = true
	}
	var b bytes.Buffer
	for _, line := range strings.Split(string(raw), "\n") {
		head := strings.SplitN(line, " ", 2)[0]
		// known_hosts entries can be `<host>` or `<host>,<host>,...` or
		// hashed (|1|...). Only drop explicit IP prefixes — leave hashed
		// entries alone (their original purpose is privacy).
		matched := false
		for _, h := range strings.Split(head, ",") {
			if ipSet[h] {
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, bytes.TrimRight(b.Bytes(), "\n"), 0o600)
}

// readGatewayDomain reads identity_domain from this env's slice in env-values.yml.
func readGatewayDomain(projectRoot, envName string) (string, error) {
	slice, err := envvalues.Get(projectRoot, envName)
	if err != nil {
		return "", err
	}
	gw := slice.String("identity_domain")
	if gw == "" {
		return "", fmt.Errorf("identity_domain not set for env %q in %s", envName, envvalues.Path(projectRoot))
	}
	return gw, nil
}

// runAnsibleBootstrap runs ansible-playbook bootstrap.yml against the
// runner image with the env-aware -e flags. Mirrors deploy's invocation
// shape minus the apps/tags selector.
func runAnsibleBootstrap(p *project.Project, envName, envDir, gateway string) error {
	inv, err := baseInvocation(p)
	if err != nil {
		return err
	}
	inv.Workdir = playbookDir
	inv.Entrypoint = "ansible-playbook"
	inv.Cmd = []string{
		"bootstrap.yml",
		"-i", containerWorkspace + "/" + p.Config.Inventory,
		"-e", "env_name=" + envName,
		"-e", "@" + containerWorkspace + "/.ansible-vars.json",
		"-e", "identity_domain=" + gateway,
	}
	if globals.Verbose {
		inv.Cmd = append(inv.Cmd, "-v")
	}
	return inv.Command().Run()
}

func waitLECert(envName, envDir, gateway string, hostIPs []string) error {
	probeURL := "https://" + gateway + "/api/v3/root/config/"
	identityIP := pickIdentityIP(envDir, hostIPs)
	restarted := false
	return wait.HTTPStatus(probeURL, 200, wait.DefaultHTTP(), func(attempt int, err error) {
		// at attempt 6 (~60s in), nudge traefik in case LE didn't trigger
		if attempt == 6 && identityIP != "" && !restarted {
			fmt.Fprintf(os.Stderr, "    LE cert not yet valid after 60s — forcing traefik restart on %s\n", identityIP)
			cmd := exec.Command("ssh",
				"-o", "StrictHostKeyChecking=accept-new",
				"-o", "BatchMode=yes",
				"root@"+identityIP,
				"docker compose -f /opt/traefik/docker-compose.yml restart traefik",
			)
			cmd.Stdout = nil
			cmd.Stderr = nil
			_ = cmd.Run()
			restarted = true
		}
	})
}

// pickIdentityIP reads the env's inventory.ini and returns the first
// IP under [identity]. Falls back to the first hostIP if no [identity]
// group is found.
func pickIdentityIP(envDir string, hostIPs []string) string {
	raw, err := os.ReadFile(filepath.Join(envDir, "inventory.ini"))
	if err != nil {
		if len(hostIPs) > 0 {
			return hostIPs[0]
		}
		return ""
	}
	in := false
	for _, line := range strings.Split(string(raw), "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "[identity]") {
			in = true
			continue
		}
		if strings.HasPrefix(l, "[") {
			in = false
			continue
		}
		if !in || l == "" {
			continue
		}
		// look for ansible_host=<ip>
		for _, f := range strings.Fields(l) {
			if strings.HasPrefix(f, "ansible_host=") {
				return strings.TrimPrefix(f, "ansible_host=")
			}
		}
	}
	if len(hostIPs) > 0 {
		return hostIPs[0]
	}
	return ""
}

// requiredFlows lists every default-* slug platform/identity/terraform's
// data sources resolve. All must be indexed before configure.sh's full
// apply runs. Kept in sync with up.sh's REQUIRED_FLOWS.
var requiredFlows = []string{
	"default-source-authentication",
	"default-source-enrollment",
	"default-invalidation-flow",
	"default-user-settings-flow",
	"default-provider-authorization-implicit-consent",
	"default-provider-invalidation-flow",
}

// waitAuthentikIndexing polls Authentik for the canonical default flows
// + RBAC view_application permission, the same gating up.sh did before
// declaring itself done.
func waitAuthentikIndexing(scwClient *scw.Client, gateway string, tfOut []byte) error {
	token, err := readAuthentikAdminToken(scwClient, tfOut)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(150 * time.Second)
	for time.Now().Before(deadline) {
		ok := true
		for _, slug := range requiredFlows {
			if !authentikSlugIndexed(gateway, token, slug) {
				ok = false
				break
			}
		}
		if ok && authentikPermissionIndexed(gateway, token, "view_application") {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	fmt.Fprintln(os.Stderr, "    warning: blueprints/RBAC not fully indexed after 150s — configure may need a retry")
	return nil
}

func authentikGet(gateway, token, path string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, "https://"+gateway+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := authentikHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("authentik %s: %d", path, resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func authentikHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func authentikSlugIndexed(gateway, token, slug string) bool {
	doc, err := authentikGet(gateway, token, "/api/v3/flows/instances/?slug="+slug)
	if err != nil {
		return false
	}
	return paginationCount(doc) >= 1
}

func authentikPermissionIndexed(gateway, token, codename string) bool {
	doc, err := authentikGet(gateway, token, "/api/v3/rbac/permissions/?codename="+codename)
	if err != nil {
		return false
	}
	return paginationCount(doc) >= 1
}

func paginationCount(doc map[string]any) int {
	p, ok := doc["pagination"].(map[string]any)
	if !ok {
		return 0
	}
	n, ok := p["count"].(float64)
	if !ok {
		return 0
	}
	return int(n)
}

// readAuthentikAdminToken extracts authentik_admin_secret_id from TF
// output, accesses the scaleway secret, and pulls .api_token from the
// JSON-encoded payload.
func readAuthentikAdminToken(scwClient *scw.Client, tfOut []byte) (string, error) {
	var doc map[string]struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(tfOut, &doc); err != nil {
		return "", err
	}
	secretRef, ok := doc["authentik_admin_secret_id"]
	if !ok || secretRef.Value == "" {
		return "", fmt.Errorf("authentik_admin_secret_id not in TF output — did the up phase complete?")
	}
	// scaleway emits "<region>/<uuid>"; CLI wants the bare UUID.
	uuid := secretRef.Value
	if idx := strings.LastIndex(uuid, "/"); idx >= 0 {
		uuid = uuid[idx+1:]
	}
	data, err := scwClient.AccessVersion(uuid, "latest")
	if err != nil {
		return "", fmt.Errorf("access admin secret: %w", err)
	}
	var payload struct {
		APIToken string `json:"api_token"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", fmt.Errorf("parse admin secret payload: %w", err)
	}
	if payload.APIToken == "" {
		return "", fmt.Errorf("api_token empty in admin secret")
	}
	return payload.APIToken, nil
}

// reconcileOutpost imports the auto-created Authentik embedded outpost
// into TF state if needed. Mirrors configure.sh's outpost-import step.
func reconcileOutpost(tfClient *tf.Client, envDir, gateway, adminToken string) error {
	const addr = `module.stack.module.identity.authentik_outpost.embedded[0]`
	inState, err := tfClient.StateShow(envDir, addr)
	if err != nil {
		return err
	}
	if inState {
		return nil
	}
	doc, err := authentikGet(gateway, adminToken, "/api/v3/outposts/instances/?name=authentik%20Embedded%20Outpost")
	if err != nil {
		// no outpost or no forward-auth apps enabled — silent no-op.
		return nil
	}
	results, ok := doc["results"].([]any)
	if !ok || len(results) == 0 {
		return nil
	}
	first, ok := results[0].(map[string]any)
	if !ok {
		return nil
	}
	pk, ok := first["pk"].(string)
	if !ok || pk == "" {
		return nil
	}
	if err := tfClient.Import(envDir, addr, pk, tf.ImportOpts{
		Vars: map[string]string{"authentik_admin_token": adminToken},
	}); err != nil {
		return fmt.Errorf("import embedded outpost: %w", err)
	}
	return nil
}
