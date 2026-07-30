package cli

type completionValue int

const (
	completeNone completionValue = iota
	completeText
	completeRateID
	completeProjectID
	completeTaskID
	completeEntryID
	completeActiveEntryID
	completePaymentID
)

type optionSpec struct {
	name    string
	boolean bool
	value   completionValue
}

type commandSpec struct {
	usage      string
	options    []optionSpec
	positional completionValue
}

type resourceSpec struct {
	actions  []string
	commands map[string]commandSpec
}

var resourceOrder = []string{
	"rates",
	"projects",
	"tasks",
	"entries",
	"payments",
}

var commandSpecs = map[string]resourceSpec{
	"rates": {
		actions: []string{"list", "get", "create", "update", "delete"},
		commands: map[string]commandSpec{
			"list": {usage: "chankat rates list"},
			"get": {
				usage: "chankat rates get ID", positional: completeRateID,
			},
			"create": {
				usage:   "chankat rates create --name NAME --amount-minor N --currency CODE",
				options: options("name", "amount-minor", "currency"),
			},
			"update": {
				usage:      "chankat rates update ID [--name NAME] [--amount-minor N] [--currency CODE]",
				positional: completeRateID,
				options:    options("name", "amount-minor", "currency"),
			},
			"delete": {
				usage: "chankat rates delete ID", positional: completeRateID,
			},
		},
	},
	"projects": {
		actions: []string{"list", "get", "create", "update", "delete"},
		commands: map[string]commandSpec{
			"list": {usage: "chankat projects list"},
			"get": {
				usage: "chankat projects get ID", positional: completeProjectID,
			},
			"create": {
				usage: "chankat projects create --name NAME --rate ID",
				options: []optionSpec{
					{name: "name"},
					{name: "rate", value: completeRateID},
				},
			},
			"update": {
				usage:      "chankat projects update ID [--name NAME] [--rate ID]",
				positional: completeProjectID,
				options: []optionSpec{
					{name: "name"},
					{name: "rate", value: completeRateID},
				},
			},
			"delete": {
				usage: "chankat projects delete ID", positional: completeProjectID,
			},
		},
	},
	"tasks": {
		actions: []string{"list", "get", "create", "update", "delete", "start"},
		commands: map[string]commandSpec{
			"list": {
				usage: "chankat tasks list [--active]",
				options: []optionSpec{
					{name: "active", boolean: true},
				},
			},
			"get": {
				usage: "chankat tasks get ID", positional: completeTaskID,
			},
			"create": {
				usage: "chankat tasks create --name NAME --project ID [--start | --started-at TIME [--ended-at TIME]] [--note TEXT]",
				options: []optionSpec{
					{name: "name"},
					{name: "project", value: completeProjectID},
					{name: "start", boolean: true},
					{name: "started-at"},
					{name: "ended-at"},
					{name: "note"},
				},
			},
			"update": {
				usage:      "chankat tasks update ID [--name NAME] [--project ID]",
				positional: completeTaskID,
				options: []optionSpec{
					{name: "name"},
					{name: "project", value: completeProjectID},
				},
			},
			"delete": {
				usage: "chankat tasks delete ID", positional: completeTaskID,
			},
			"start": {
				usage:      "chankat tasks start ID [--at TIME]",
				positional: completeTaskID,
				options:    options("at"),
			},
		},
	},
	"entries": {
		actions: []string{"list", "get", "create", "update", "delete", "stop"},
		commands: map[string]commandSpec{
			"list": {
				usage: "chankat entries list [--task ID] [--active]",
				options: []optionSpec{
					{name: "task", value: completeTaskID},
					{name: "active", boolean: true},
				},
			},
			"get": {
				usage: "chankat entries get ID", positional: completeEntryID,
			},
			"create": {
				usage: "chankat entries create --task ID --started-at TIME [--ended-at TIME] [--note TEXT]",
				options: []optionSpec{
					{name: "task", value: completeTaskID},
					{name: "started-at"},
					{name: "ended-at"},
					{name: "note"},
				},
			},
			"update": {
				usage:      "chankat entries update ID [--started-at TIME] [--ended-at TIME] [--note TEXT]",
				positional: completeEntryID,
				options:    options("started-at", "ended-at", "note"),
			},
			"delete": {
				usage: "chankat entries delete ID", positional: completeEntryID,
			},
			"stop": {
				usage:      "chankat entries stop ID | chankat entries stop --all",
				positional: completeActiveEntryID,
				options: []optionSpec{
					{name: "all", boolean: true},
				},
			},
		},
	},
	"payments": {
		actions: []string{"list", "get", "create", "update", "delete"},
		commands: map[string]commandSpec{
			"list": {usage: "chankat payments list"},
			"get": {
				usage: "chankat payments get ID", positional: completePaymentID,
			},
			"create": {
				usage: "chankat payments create --project ID --amount-minor N --currency CODE [--paid-at DATE] [--paid-for DATE] [--note TEXT]",
				options: []optionSpec{
					{name: "project", value: completeProjectID},
					{name: "amount-minor"},
					{name: "currency"},
					{name: "paid-at"},
					{name: "paid-for"},
					{name: "note"},
				},
			},
			"update": {
				usage:      "chankat payments update ID [--project ID] [--amount-minor N] [--currency CODE] [--paid-at DATE] [--paid-for DATE] [--note TEXT]",
				positional: completePaymentID,
				options: []optionSpec{
					{name: "project", value: completeProjectID},
					{name: "amount-minor"},
					{name: "currency"},
					{name: "paid-at"},
					{name: "paid-for"},
					{name: "note"},
				},
			},
			"delete": {
				usage: "chankat payments delete ID", positional: completePaymentID,
			},
		},
	},
}

func command(resource, action string) (commandSpec, bool) {
	spec, ok := commandSpecs[resource]
	if !ok {
		return commandSpec{}, false
	}
	command, ok := spec.commands[action]
	return command, ok
}

func options(names ...string) []optionSpec {
	result := make([]optionSpec, len(names))
	for i, name := range names {
		result[i] = optionSpec{name: name}
	}
	return result
}

func (o optionSpec) completionValue() completionValue {
	if o.boolean {
		return completeNone
	}
	if o.value != completeNone {
		return o.value
	}
	return completeText
}
