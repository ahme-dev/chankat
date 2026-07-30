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
	if filtered[0] == "version" || filtered[0] == "help" {
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
	resourceHelp := map[string]string{
		"rates":    "list|get|create|update|delete",
		"projects": "list|get|create|update|delete",
		"tasks":    "list|get|create|update|delete|start",
		"entries":  "list|get|create|update|delete|stop",
		"payments": "list|get|create|update|delete",
	}
	if len(command) == 1 {
		if actions, ok := resourceHelp[command[0]]; ok {
			fmt.Fprintf(r.out, "Usage: chankat %s %s\n", command[0], actions)
			return
		}
		r.usage()
		return
	}
	usages := map[string]string{
		"rates list":      "chankat rates list",
		"rates get":       "chankat rates get ID",
		"rates create":    "chankat rates create --name NAME --amount-minor N --currency CODE",
		"rates update":    "chankat rates update ID [--name NAME] [--amount-minor N] [--currency CODE]",
		"rates delete":    "chankat rates delete ID",
		"projects list":   "chankat projects list",
		"projects get":    "chankat projects get ID",
		"projects create": "chankat projects create --name NAME --rate ID",
		"projects update": "chankat projects update ID [--name NAME] [--rate ID]",
		"projects delete": "chankat projects delete ID",
		"tasks list":      "chankat tasks list [--active]",
		"tasks get":       "chankat tasks get ID",
		"tasks create":    "chankat tasks create --name NAME --project ID [--start | --started-at TIME [--ended-at TIME]] [--note TEXT]",
		"tasks update":    "chankat tasks update ID [--name NAME] [--project ID]",
		"tasks delete":    "chankat tasks delete ID",
		"tasks start":     "chankat tasks start ID [--at TIME]",
		"entries list":    "chankat entries list [--task ID] [--active]",
		"entries get":     "chankat entries get ID",
		"entries create":  "chankat entries create --task ID --started-at TIME [--ended-at TIME] [--note TEXT]",
		"entries update":  "chankat entries update ID [--started-at TIME] [--ended-at TIME] [--note TEXT]",
		"entries delete":  "chankat entries delete ID",
		"entries stop":    "chankat entries stop ID | chankat entries stop --all",
		"payments list":   "chankat payments list",
		"payments get":    "chankat payments get ID",
		"payments create": "chankat payments create --project ID --amount-minor N --currency CODE [--paid-at DATE] [--paid-for DATE] [--note TEXT]",
		"payments update": "chankat payments update ID [--project ID] [--amount-minor N] [--currency CODE] [--paid-at DATE] [--paid-for DATE] [--note TEXT]",
		"payments delete": "chankat payments delete ID",
	}
	key := strings.Join(command, " ")
	if usage, ok := usages[key]; ok {
		fmt.Fprintln(r.out, "Usage: "+usage)
		return
	}
	fmt.Fprintf(r.out, "unknown command %q\n", key)
}

func (r runner) usage() {
	fmt.Fprint(r.out, `Usage:
  chankat                         launch the terminal interface
  chankat [--json] <resource> <command> [options]
  chankat version

Resources:
  rates       list, get, create, update, delete
  projects    list, get, create, update, delete
  tasks       list, get, create, update, delete, start
  entries     list, get, create, update, delete, stop
  payments    list, get, create, update, delete

Run 'chankat <resource> <command> --help' for command options.
`)
}

func (r runner) flags(name, usage string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(r.errOut)
	flags.Usage = func() {
		fmt.Fprintln(r.errOut, usage)
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
