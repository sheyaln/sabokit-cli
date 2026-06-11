package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/envvalues"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/sheyaln/sabokit-cli/internal/scw"
	"github.com/spf13/cobra"
)

type upFlags struct {
	skipPreflight bool
	layers        []string
	noConfirm     bool
	dryRun        bool
}

func newUpCmd() *cobra.Command {
	f := &upFlags{}
	cmd := &cobra.Command{
		Use:   "up",
		Short: "provision + deploy the current env end-to-end (all four layers)",
		Long: `runs the full bring-up for environments/<env>/: preflight, then the
four-layer scripts in dependency order inside the sabokit-runner image.

  preflight (host-side, pure Go):
    env.yml present with required keys, no two envs share a project_id,
    SCW creds reach the project, ssh public key in scaleway IAM,
    base_domain's DNS zone registered with scaleway,
    per-layer backend.hcl present (generated when missing),
    the TF-state bucket exists (created when missing).

  layers (inside the runner; terraform + ansible + scw baked in):
    infra        scaleway substrate — VPC, hosts, Postgres, TEM, DNS, secrets
    identity     ansible boots Authentik, then terraform configures it
                 (waits for SSH, DNS propagation, the LE cert, and Authentik
                 flow/RBAC indexing in between)
    operations   observability DBs + OIDC apps, then the ops containers
    application  per-app DBs + OIDC + outpost, then every enabled app

the scripts are the runbook (consumer-template/scripts/); the CLI runs the
consumer's vendored scripts/ when present, else the copy baked into the
runner image at the env's pinned blueprint tag. each layer is idempotent —
re-run up (or a single layer via --layers) after a failure and it resumes.

every terraform apply inside the scripts is auto-approved; the CLI gates
the whole run with one confirmation up front (default yes in non-prod
envs, no in prod). --no-confirm bypasses it for unattended runs.`,
		Example: `  sabokit up                            # full first deploy
  sabokit --env staging up              # against another env
  sabokit up --layers application       # one layer (fast app churn)
  sabokit up --layers operations,application
  sabokit up --no-confirm               # unattended`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUp(f)
		},
	}
	cmd.Flags().BoolVar(&f.skipPreflight, "skip-preflight", false, "skip the host-side checks (creds, ssh key, DNS zone, backends)")
	cmd.Flags().StringSliceVar(&f.layers, "layers", nil, "subset of layers to run, in order (default: all four)")
	cmd.Flags().BoolVar(&f.noConfirm, "no-confirm", false, "skip the confirmation gate (required for unattended runs)")
	cmd.Flags().BoolVar(&f.dryRun, "print", false, "print the docker invocation(s) without running them")
	return cmd
}

func runUp(f *upFlags) error {
	layers, err := resolveLayers(f.layers)
	if err != nil {
		return err
	}
	p, err := project.Load()
	if err != nil {
		return err
	}
	envName := p.EnvName(globals.Env)
	if envName == "" {
		return fmt.Errorf("up requires an env (pass --env or set default_env in .sabokit/config.yml)")
	}
	if !p.IsFourLayer(globals.Env) {
		return fmt.Errorf("environments/%s is not a four-layer env (no env.yml + infra/stack.tf) — scaffold with 'sabokit env add' or migrate it to blueprint v0.2", envName)
	}
	if err := requireCompatibleBlueprint(p); err != nil {
		return err
	}

	if f.dryRun {
		for _, layer := range layers {
			inv, err := scriptInvocation(p, layer+".sh", envName)
			if err != nil {
				return err
			}
			fmt.Println("docker", strings.Join(inv.Args(), " "))
		}
		return nil
	}

	if err := docker.Preflight(); err != nil {
		return err
	}
	envDir, err := p.WorkspaceDir(globals.Env)
	if err != nil {
		return err
	}
	values, err := envvalues.ForEnv(p.Root, envName)
	if err != nil {
		return err
	}

	if !f.skipPreflight {
		if err := runPreflight(p, envName, envDir, values); err != nil {
			return err
		}
	} else {
		phase("skipping preflight (--skip-preflight)")
	}

	if !f.noConfirm {
		ok, err := confirmUp(envName, values.String("scaleway_project_id"), layers, os.Stdin)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("aborted by user")
		}
	}

	for i, layer := range layers {
		phase(fmt.Sprintf("%d/%d %s", i+1, len(layers), layer))
		if err := runScript(p, layer+".sh", envName); err != nil {
			return fmt.Errorf("%s layer: %w", layer, err)
		}
	}

	phase("done")
	return nil
}

func phase(msg string) {
	fmt.Printf("==> %s\n", msg)
}

// resolveLayers validates an ordered subset of the canonical layer list,
// defaulting to all four. Order is normalised to dependency order regardless
// of how the flag spelled it.
func resolveLayers(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return project.Layers, nil
	}
	want := map[string]bool{}
	for _, l := range requested {
		l = strings.TrimSpace(l)
		valid := false
		for _, known := range project.Layers {
			if l == known {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("unknown layer %q (valid: %s)", l, strings.Join(project.Layers, ", "))
		}
		want[l] = true
	}
	out := make([]string, 0, len(want))
	for _, l := range project.Layers {
		if want[l] {
			out = append(out, l)
		}
	}
	return out, nil
}

// confirmUp prompts once before any terraform runs. Default is yes for
// non-prod envs (Enter accepts), no for prod (Enter aborts).
func confirmUp(envName, projectID string, layers []string, in io.Reader) (bool, error) {
	fmt.Printf("about to apply layer(s) %s to env %q (scaleway project %s)\n",
		strings.Join(layers, " → "), envName, projectID)
	isProd := envName == "prod"
	if isProd {
		fmt.Print("proceed? [y/N] ")
	} else {
		fmt.Print("proceed? [Y/n] ")
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

// runPreflight verifies the env can deploy: required values present and
// distinct across envs, SCW creds reach the project, the user's SSH key is
// in the project's IAM keystore, the DNS zone is delegated, and every
// layer's backend.hcl + the state bucket exist (both created when missing).
func runPreflight(p *project.Project, envName, envDir string, values envvalues.Slice) error {
	phase("preflight")

	all, err := envvalues.LoadResolved(p.Root)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if err := envvalues.CheckDistinctProjectIDs(all); err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if err := values.Require(envvalues.RequiredKeys...); err != nil {
		return fmt.Errorf("preflight: %s/env.yml: %w", envDir, err)
	}
	projectID := values.String("scaleway_project_id")
	baseDomain := values.String("base_domain")
	gateway := values.String("identity_domain")
	if !strings.HasSuffix(gateway, "."+baseDomain) && gateway != baseDomain {
		fmt.Fprintf(os.Stderr, "    warning: identity_domain %q does not sit under base_domain %q — terraform may refuse to manage the A record\n", gateway, baseDomain)
	}

	if os.Getenv("SCW_ACCESS_KEY") == "" || os.Getenv("SCW_SECRET_KEY") == "" {
		return fmt.Errorf("preflight: SCW_ACCESS_KEY and SCW_SECRET_KEY must be exported in the environment")
	}

	scwClient := scw.New(globals.ScwImage, globals.Platform)
	if err := scwClient.AccountProjectGet(projectID); err != nil {
		return fmt.Errorf("preflight: scw cannot reach project %s with the current credentials: %w", projectID, err)
	}
	if err := ensureSSHKeyUploaded(p, envName, projectID, scwClient); err != nil {
		return err
	}
	if err := ensureDNSZoneDelegated(baseDomain, scwClient); err != nil {
		return err
	}
	return ensureLayerBackends(p, envName, envDir, values, scwClient)
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

var backendBucketRe = regexp.MustCompile(`(?m)^\s*bucket\s*=\s*"([^"]+)"`)

// ensureLayerBackends guarantees each layer root has a backend.hcl and that
// the bucket they point at exists. Missing files are generated against the
// env's canonical bucket: the one any existing layer already uses, else
// "<org>-tfstate-<env>". One bucket per env, one state key per layer.
func ensureLayerBackends(p *project.Project, envName, envDir string, values envvalues.Slice, scwClient *scw.Client) error {
	bucket := ""
	missing := []string{}
	for _, layer := range project.Layers {
		path := filepath.Join(envDir, layer, "backend.hcl")
		raw, err := os.ReadFile(path)
		if err != nil {
			missing = append(missing, layer)
			continue
		}
		m := backendBucketRe.FindSubmatch(raw)
		if m == nil {
			return fmt.Errorf("preflight: %s has no bucket = \"...\" line", path)
		}
		if b := string(m[1]); bucket == "" {
			bucket = b
		} else if b != bucket {
			return fmt.Errorf("preflight: layer backends disagree on the state bucket (%q vs %q) — one bucket per env, one key per layer", bucket, b)
		}
	}
	if bucket == "" {
		orgSlug, err := readOrgSlug(p.Root)
		if err != nil {
			return fmt.Errorf("preflight: derive state bucket name: %w", err)
		}
		bucket = fmt.Sprintf("%s-tfstate-%s", orgSlug, envName)
	}
	for _, layer := range missing {
		path := filepath.Join(envDir, layer, "backend.hcl")
		fmt.Printf("    writing %s (bucket %s)\n", path, bucket)
		if err := writeLayerBackendHCL(envDir, layer, bucket); err != nil {
			return err
		}
	}
	region := values.String("scaleway_region")
	if region == "" {
		region = p.Config.Scaleway.Region
	}
	if region == "" {
		region = "fr-par"
	}
	if err := scwClient.CreateBucket(bucket, region); err != nil {
		return fmt.Errorf("preflight: ensure state bucket %s: %w", bucket, err)
	}
	return nil
}

// writeLayerBackendHCL materialises one layer's backend.hcl — the same shape
// init/env add generate. Kept in sync deliberately.
func writeLayerBackendHCL(envDir, layer, bucket string) error {
	content := fmt.Sprintf(`# Generated by sabokit. One bucket per env, one state key per layer.

bucket = %q
key    = "%s/terraform.tfstate"
`, bucket, layer)
	return os.WriteFile(filepath.Join(envDir, layer, "backend.hcl"), []byte(content), 0o644)
}
