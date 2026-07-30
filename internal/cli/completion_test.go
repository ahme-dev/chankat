package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionCommand(t *testing.T) {
	var out bytes.Buffer
	if err := RunIO(
		t.Context(),
		[]string{"completion", "bash"},
		"test",
		nil,
		&out,
		&out,
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `" __complete "`) ||
		!strings.Contains(out.String(), "complete -F") {
		t.Fatalf("unexpected Bash completion:\n%s", out.String())
	}
	if RequiresStorage([]string{"completion", "bash"}) {
		t.Fatal("completion script generation requires storage")
	}
}

func TestStaticCompletion(t *testing.T) {
	for _, test := range []struct {
		args []string
		want []string
	}{
		{args: []string{""}, want: []string{"rates", "completion", "--json"}},
		{args: []string{"--json", "pro"}, want: []string{"projects"}},
		{args: []string{"tasks", "cr"}, want: []string{"create"}},
		{args: []string{"completion", ""}, want: []string{"bash"}},
		{
			args: []string{"tasks", "create", "--"},
			want: []string{"--name", "--project", "--start", "--help"},
		},
		{
			args: []string{"entries", "stop", "--"},
			want: []string{"--all", "--help"},
		},
	} {
		request := analyzeCompletion(test.args)
		for _, want := range test.want {
			if !contains(request.candidates, want) {
				t.Errorf("%v candidates %v do not contain %q",
					test.args, request.candidates, want)
			}
		}
		if completionNeedsStorage(test.args) {
			t.Errorf("%v unexpectedly requires storage", test.args)
		}
	}
}

func TestDynamicCompletion(t *testing.T) {
	stor := cliStorage(t)
	runCLI(t, stor, "rates", "create", "--name", "Rate",
		"--amount-minor", "8000", "--currency", "USD")
	runCLI(t, stor, "projects", "create", "--name", "Project", "--rate", "1")
	runCLI(t, stor, "tasks", "create", "--name", "Active", "--project", "1",
		"--started-at", "2026-07-30T09:00:00Z")
	runCLI(t, stor, "tasks", "create", "--name", "Finished", "--project", "1",
		"--started-at", "2026-07-30T09:00:00Z",
		"--ended-at", "2026-07-30T10:00:00Z")

	for _, test := range []struct {
		args []string
		want []string
		not  []string
	}{
		{
			args: []string{"projects", "create", "--rate", ""},
			want: []string{"1"},
		},
		{
			args: []string{"tasks", "update", ""},
			want: []string{"1", "2", "--name", "--project"},
		},
		{
			args: []string{"entries", "stop", ""},
			want: []string{"1", "--all"},
			not:  []string{"2"},
		},
		{
			args: []string{"projects", "create", "--rate="},
			want: []string{"--rate=1"},
		},
	} {
		output := runCLI(
			t,
			stor,
			append([]string{"__complete"}, test.args...)...,
		)
		candidates := strings.Fields(output)
		for _, want := range test.want {
			if !contains(candidates, want) {
				t.Errorf("%v candidates %v do not contain %q",
					test.args, candidates, want)
			}
		}
		for _, unwanted := range test.not {
			if contains(candidates, unwanted) {
				t.Errorf("%v candidates %v contain %q",
					test.args, candidates, unwanted)
			}
		}
		if !RequiresStorage(append([]string{"__complete"}, test.args...)) {
			t.Errorf("%v should require storage", test.args)
		}
	}
}

func TestTextOptionHasNoValueCompletion(t *testing.T) {
	request := analyzeCompletion([]string{
		"tasks", "create", "--name", "",
	})
	if request.value != completeText || len(request.candidates) != 0 {
		t.Fatalf("unexpected request: %#v", request)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
