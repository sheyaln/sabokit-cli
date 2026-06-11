package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/sheyaln/sabokit-cli/internal/docker"
	"github.com/sheyaln/sabokit-cli/internal/project"
	"github.com/spf13/cobra"
)

type destroyFlags struct {
	layer  string
	all    bool
	yes    bool
	dryRun bool
}

func newDestroyCmd() *cobra.Command {
	f := &destroyFlags{}
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "terraform destroy in the current env (per-layer or full)",
		Long: `tears down cloud state via the layer scripts inside the runner image.

modes (exactly one required):
  --layer L    scripts/destroy-layer.sh <env> L — one layer
  --all        scripts/down.sh <env> — every layer, reverse dependency
               order (application → operations → identity → infra)

to remove ONE app, don't destroy: disable it in environments/<env>/
application.yml and run 'sabokit up --layers application' — terraform
removes its resources declaratively.

confirms before acting unless --yes is passed.`,
		Example: `  sabokit destroy --layer application
  sabokit destroy --all
  sabokit --env staging destroy --all --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDestroy(f)
		},
	}
	cmd.Flags().StringVar(&f.layer, "layer", "", "destroy one layer: "+strings.Join(project.Layers, " | "))
	cmd.Flags().BoolVar(&f.all, "all", false, "destroy everything in the env (all four layers, reverse order)")
	cmd.Flags().BoolVarP(&f.yes, "yes", "y", false, "skip the confirmation prompt")
	cmd.Flags().BoolVar(&f.dryRun, "print", false, "print the docker invocation without running it")
	return cmd
}

func runDestroy(f *destroyFlags) error {
	if (f.layer != "") == f.all {
		return fmt.Errorf("exactly one of --layer, --all is required")
	}
	if f.layer != "" {
		valid := false
		for _, l := range project.Layers {
			if f.layer == l {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("--layer must be one of: %s", strings.Join(project.Layers, ", "))
		}
	}

	p, err := project.Load()
	if err != nil {
		return err
	}
	envName := p.EnvName(globals.Env)
	if envName == "" {
		return fmt.Errorf("destroy requires an env (pass --env or set default_env)")
	}
	if err := requireCompatibleBlueprint(p); err != nil {
		return err
	}

	script, args := "down.sh", []string{envName}
	summary := fmt.Sprintf("destroy EVERYTHING in env %s (all four layers)", envName)
	if f.layer != "" {
		script, args = "destroy-layer.sh", []string{envName, f.layer}
		summary = fmt.Sprintf("destroy layer %s in env %s", f.layer, envName)
	}

	if f.dryRun {
		inv, err := scriptInvocation(p, script, args...)
		if err != nil {
			return err
		}
		fmt.Println("docker", strings.Join(inv.Args(), " "))
		return nil
	}

	if !f.yes {
		fmt.Printf("about to %s\n", summary)
		fmt.Print("proceed? [y/N] ")
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		line = strings.ToLower(strings.TrimSpace(line))
		if line != "y" && line != "yes" {
			return fmt.Errorf("aborted")
		}
	}

	if err := docker.Preflight(); err != nil {
		return err
	}
	return runScript(p, script, args...)
}
