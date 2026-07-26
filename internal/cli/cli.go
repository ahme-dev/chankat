package cli

import "fmt"

func Run(args []string) error {
	switch {
	case len(args) == 0:
		return nil
	case args[0] == "":
		return fmt.Errorf("expected command, got nothing")
	case args[0] == "version":
		return fmt.Errorf("version not implemented")
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
