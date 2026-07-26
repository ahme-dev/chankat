package cli

import "fmt"

func Run(args []string, version string) error {
	switch {
	case len(args) == 0:
		return nil
	case args[0] == "":
		return fmt.Errorf("expected command, got nothing")
	case args[0] == "version":
		fmt.Printf("chansat %s\n", version)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
