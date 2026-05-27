package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

type destroyFlags struct {
	apps   []string
	layer  string
	all    bool
	yes    bool
	dryRun bool
}

func newDestroyCmd() *cobra.Command {
	f := &destroyFlags{}
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "terraform destroy in the current env (per-app, per-layer, or full)",
		Long: `runs terraform destroy LOCALLY in environments/<env>/ with optional
targeting.

modes (exactly one required):
  --apps X[,Y]    -target=module.stack.module.<each>  for each named app
  --layer L       -target=module.stack.module.<L>     for L in base|identity|apps
  --all           no -target; destroy the whole env

destroys cloud state; the TF state file is updated. confirms before
acting unless --yes is passed. requires terraform on PATH and SCW_*
env vars set.`,
		Example: `  sabokit destroy --apps espocrm,n8n
  sabokit destroy --layer apps
  sabokit destroy --all --yes
  sabokit destroy --apps espocrm --print`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDestroy(f)
		},
	}
	cmd.Flags().StringSliceVar(&f.apps, "apps", nil, "destroy specific apps (-target=module.stack.module.<app>)")
	cmd.Flags().StringVar(&f.layer, "layer", "", "destroy a whole layer: base | identity | apps")
	cmd.Flags().BoolVar(&f.all, "all", false, "destroy everything in the env (no -target)")
	cmd.Flags().BoolVarP(&f.yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&f.dryRun, "print", false, "print the terraform invocation without running it")
	return cmd
}

func runDestroy(f *destroyFlags) error {
	modes := 0
	if len(f.apps) > 0 {
		modes++
	}
	if f.layer != "" {
		modes++
	}
	if f.all {
		modes++
	}
	if modes != 1 {
		return fmt.Errorf("exactly one of --apps, --layer, --all is required")
	}
	if f.layer != "" {
		switch f.layer {
		case "base", "identity", "apps":
		default:
			return fmt.Errorf("--layer must be one of: base, identity, apps")
		}
	}

	p, err := project.Load()
	if err != nil {
		return err
	}
	if p.EnvName(globals.Env) == "" {
		return fmt.Errorf("destroy requires an env (pass --env or set default_env)")
	}
	envDir, err := p.WorkspaceDir(globals.Env)
	if err != nil {
		return err
	}

	args := []string{"destroy"}
	if !f.yes {
		// confirm interactively below; tf -auto-approve only when --yes
	} else {
		args = append(args, "-auto-approve")
	}
	args = append(args, "-input=false")
	for _, target := range targetsForDestroy(f) {
		args = append(args, "-target="+target)
	}

	if f.dryRun {
		fmt.Println("terraform", strings.Join(args, " "), "(in", envDir+")")
		return nil
	}

	if !f.yes {
		summary := destroySummary(f, p.EnvName(globals.Env))
		fmt.Printf("about to %s\n", summary)
		fmt.Print("proceed? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "y" && line != "yes" {
			return fmt.Errorf("aborted")
		}
		args = append(args, "-auto-approve")
	}

	if _, err := exec.LookPath("terraform"); err != nil {
		return fmt.Errorf("terraform not found in PATH")
	}
	c := exec.Command("terraform", args...)
	c.Dir = envDir
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func targetsForDestroy(f *destroyFlags) []string {
	switch {
	case len(f.apps) > 0:
		out := make([]string, 0, len(f.apps))
		for _, a := range f.apps {
			out = append(out, "module.stack.module."+a)
		}
		return out
	case f.layer != "":
		return []string{"module.stack.module." + f.layer}
	}
	return nil
}

func destroySummary(f *destroyFlags, env string) string {
	switch {
	case len(f.apps) > 0:
		return fmt.Sprintf("destroy apps %s in env %s", strings.Join(f.apps, ","), env)
	case f.layer != "":
		return fmt.Sprintf("destroy layer %s in env %s", f.layer, env)
	case f.all:
		return fmt.Sprintf("destroy EVERYTHING in env %s", env)
	}
	return "destroy"
}
