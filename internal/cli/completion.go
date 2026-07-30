package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"chankat/internal/storage"
)

const bashCompletion = `_chankat_completion()
{
	local candidate command
	command="${COMP_WORDS[0]}"
	COMPREPLY=()

	while IFS= read -r candidate; do
		COMPREPLY+=("$candidate")
	done < <("$command" __complete "${COMP_WORDS[@]:1}" 2>/dev/null)
}

complete -F _chankat_completion chankat
`

type completionRequest struct {
	prefix     string
	value      completionValue
	candidates []string
	valueFlag  string
}

func (r runner) runCompletion(args []string) error {
	if len(args) != 1 || args[0] != "bash" {
		return fmt.Errorf("usage: chankat completion bash")
	}
	_, err := fmt.Fprint(r.out, bashCompletion)
	return err
}

func (r runner) complete(args []string) error {
	request := analyzeCompletion(args)
	candidates := request.candidates
	if request.value > completeText {
		dynamic, err := r.completeIDs(request.value)
		if err != nil {
			return err
		}
		candidates = append(candidates, dynamic...)
	}
	for _, candidate := range candidates {
		if !strings.HasPrefix(candidate, request.prefix) {
			continue
		}
		if request.valueFlag != "" {
			candidate = request.valueFlag + "=" + candidate
		}
		if _, err := fmt.Fprintln(r.out, candidate); err != nil {
			return err
		}
	}
	return nil
}

func analyzeCompletion(args []string) completionRequest {
	if len(args) == 0 {
		args = []string{""}
	}
	if args[0] == "--json" {
		args = args[1:]
		if len(args) == 0 {
			args = []string{""}
		}
	}
	current := args[len(args)-1]
	if len(args) == 1 {
		return completionRequest{
			prefix: current,
			candidates: append(
				append([]string{}, resourceOrder...),
				"version", "help", "completion", "--json",
			),
		}
	}
	if args[0] == "completion" {
		if len(args) == 2 {
			return completionRequest{
				prefix: current, candidates: []string{"bash"},
			}
		}
		return completionRequest{prefix: current}
	}

	resource, ok := commandSpecs[args[0]]
	if !ok {
		return completionRequest{prefix: current}
	}
	if len(args) == 2 {
		return completionRequest{
			prefix: current, candidates: resource.actions,
		}
	}
	spec, ok := resource.commands[args[1]]
	if !ok {
		return completionRequest{prefix: current}
	}

	if option, value, ok := optionWithValue(spec, current); ok {
		return completionRequest{
			prefix: value, value: option.completionValue(),
			valueFlag: "--" + option.name,
		}
	}
	if len(args) > 3 {
		if option, ok := findOption(spec, args[len(args)-2]); ok &&
			option.completionValue() != completeNone {
			return completionRequest{
				prefix: current, value: option.completionValue(),
			}
		}
	}

	options := commandOptions(spec, args)
	if len(args) == 3 && !strings.HasPrefix(current, "-") &&
		spec.positional != completeNone {
		return completionRequest{
			prefix: current, value: spec.positional, candidates: options,
		}
	}
	return completionRequest{prefix: current, candidates: options}
}

func commandOptions(spec commandSpec, args []string) []string {
	used := make(map[string]bool)
	for _, arg := range args[2 : len(args)-1] {
		name := strings.TrimPrefix(strings.SplitN(arg, "=", 2)[0], "--")
		used[name] = true
	}
	options := make([]string, 0, len(spec.options)+1)
	for _, option := range spec.options {
		if !used[option.name] {
			options = append(options, "--"+option.name)
		}
	}
	options = append(options, "--help")
	return options
}

func optionWithValue(
	spec commandSpec,
	current string,
) (optionSpec, string, bool) {
	name, value, found := strings.Cut(current, "=")
	if !found {
		return optionSpec{}, "", false
	}
	option, ok := findOption(spec, name)
	if !ok || option.completionValue() == completeNone {
		return optionSpec{}, "", false
	}
	return option, value, true
}

func findOption(spec commandSpec, value string) (optionSpec, bool) {
	name := strings.TrimPrefix(value, "--")
	for _, option := range spec.options {
		if option.name == name {
			return option, true
		}
	}
	return optionSpec{}, false
}

func completionNeedsStorage(args []string) bool {
	request := analyzeCompletion(args)
	return request.value > completeText
}

func (r runner) completeIDs(value completionValue) ([]string, error) {
	if r.stor == nil {
		return nil, fmt.Errorf("storage is required for ID completion")
	}
	var ids []int
	switch value {
	case completeRateID:
		items, err := r.stor.GetRates(r.ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	case completeProjectID:
		items, err := r.stor.GetProjects(r.ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	case completeTaskID:
		items, err := r.stor.GetTasks(r.ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	case completeEntryID, completeActiveEntryID:
		var items []storage.Entry
		var err error
		if value == completeActiveEntryID {
			items, err = r.stor.GetActiveEntries(r.ctx)
		} else {
			items, err = r.stor.GetEntries(r.ctx)
		}
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	case completePaymentID:
		items, err := r.stor.GetPayments(r.ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	}
	slices.Sort(ids)
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = strconv.Itoa(id)
	}
	return result, nil
}
