package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"chankat/internal/storage"
)

type runner struct {
	ctx     context.Context
	stor    *storage.Storage
	version string
	out     io.Writer
	errOut  io.Writer
	json    bool
	now     func() time.Time
}

func Run(
	ctx context.Context,
	args []string,
	version string,
	stor *storage.Storage,
) error {
	return RunIO(ctx, args, version, stor, os.Stdout, os.Stderr)
}

func RunIO(
	ctx context.Context,
	args []string,
	version string,
	stor *storage.Storage,
	out io.Writer,
	errOut io.Writer,
) error {
	filtered, jsonOutput := globalOptions(args)
	r := runner{
		ctx: ctx, stor: stor, version: version, out: out, errOut: errOut,
		json: jsonOutput, now: time.Now,
	}
	err := r.run(filtered)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	return err
}

func RequiresStorage(args []string) bool {
	filtered, _ := globalOptions(args)
	if len(filtered) == 0 {
		return false
	}
	if filtered[0] == "__complete" {
		return completionNeedsStorage(filtered[1:])
	}
	if filtered[0] == "version" ||
		filtered[0] == "help" ||
		filtered[0] == "completion" {
		return false
	}
	if len(filtered) > 1 && filtered[1] == "help" {
		return false
	}
	return !hasHelpFlag(filtered)
}

func globalOptions(args []string) ([]string, bool) {
	jsonOutput := false
	for len(args) > 0 && args[0] == "--json" {
		jsonOutput = true
		args = args[1:]
	}
	return args, jsonOutput
}

func (r runner) run(args []string) error {
	if len(args) == 0 {
		r.usage()
		return nil
	}
	if args[0] == "__complete" {
		return r.complete(args[1:])
	}
	if hasHelpFlag(args) {
		r.help(args)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		r.usage()
		return nil
	case "version":
		if len(args) != 1 {
			return fmt.Errorf("usage: chankat version")
		}
		if r.json {
			return r.writeJSON(struct {
				Version string `json:"version"`
			}{Version: r.version})
		}
		_, err := fmt.Fprintf(r.out, "chankat %s\n", r.version)
		return err
	case "completion":
		return r.runCompletion(args[1:])
	case "rates":
		return r.runRates(args[1:])
	case "projects":
		return r.runProjects(args[1:])
	case "tasks":
		return r.runTasks(args[1:])
	case "entries":
		return r.runEntries(args[1:])
	case "payments":
		return r.runPayments(args[1:])
	default:
		return fmt.Errorf("unknown command %q; run 'chankat help'", args[0])
	}
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func (r runner) help(args []string) {
	command := make([]string, 0, 2)
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			continue
		}
		if len(command) < 2 {
			command = append(command, arg)
		}
	}
	if len(command) == 0 {
		r.usage()
		return
	}
	if command[0] == "completion" {
		fmt.Fprintln(r.out, "Usage: chankat completion bash")
		return
	}
	if len(command) == 1 {
		if resource, ok := commandSpecs[command[0]]; ok {
			fmt.Fprintf(
				r.out,
				"Usage: chankat %s %s\n",
				command[0],
				strings.Join(resource.actions, "|"),
			)
			return
		}
		r.usage()
		return
	}
	if spec, ok := commandSpecs[command[0]]; ok {
		if action, ok := spec.commands[command[1]]; ok {
			fmt.Fprintln(r.out, "Usage: "+action.usage)
			return
		}
	}
	fmt.Fprintf(r.out, "unknown command %q\n", strings.Join(command, " "))
}

func (r runner) usage() {
	fmt.Fprint(r.out, `Usage:
  chankat                         launch the terminal interface
  chankat [--json] <resource> <command> [options]
  chankat completion bash
  chankat version

Resources:
`)
	for _, name := range resourceOrder {
		resource := commandSpecs[name]
		fmt.Fprintf(r.out, "  %-12s%s\n", name, strings.Join(resource.actions, ", "))
	}
	fmt.Fprint(r.out, `
Run 'chankat <resource> <command> --help' for command options.
`)
}

func (r runner) flags(resource, action string) *flag.FlagSet {
	spec, ok := command(resource, action)
	if !ok {
		panic("unknown command metadata: " + resource + " " + action)
	}
	flags := flag.NewFlagSet(resource+" "+action, flag.ContinueOnError)
	flags.SetOutput(r.errOut)
	flags.Usage = func() {
		fmt.Fprintln(r.errOut, "Usage: "+spec.usage)
		flags.PrintDefaults()
	}
	return flags
}

func requireNoArgs(args []string, usage string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: %s", usage)
	}
	return nil
}

func parseID(value, entity string) (int, error) {
	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid %s ID %q", entity, value)
	}
	return id, nil
}

func changed(flags *flag.FlagSet, name string) bool {
	found := false
	flags.Visit(func(item *flag.Flag) {
		if item.Name == name {
			found = true
		}
	})
	return found
}

func required(flags *flag.FlagSet, names ...string) error {
	missing := make([]string, 0)
	for _, name := range names {
		if !changed(flags, name) {
			missing = append(missing, "--"+name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required option %s", strings.Join(missing, ", "))
	}
	return nil
}

func (r runner) status(action, entity string, id int) error {
	if r.json {
		return r.writeJSON(struct {
			Status string `json:"status"`
			Entity string `json:"entity"`
			ID     int    `json:"id,omitempty"`
		}{Status: action, Entity: entity, ID: id})
	}
	if id > 0 {
		_, err := fmt.Fprintf(r.out, "%s %d %s\n", entity, id, action)
		return err
	}
	_, err := fmt.Fprintf(r.out, "%s %s\n", entity, action)
	return err
}
