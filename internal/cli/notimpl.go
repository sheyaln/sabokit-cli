package cli

import "fmt"

func notImplemented(name string) error {
	return fmt.Errorf("%s: not implemented in this build", name)
}
