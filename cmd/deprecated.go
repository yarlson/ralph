package cmd

import (
	"fmt"
	"io"
)

// deprecationMessages maps deprecated command names to their replacement commands.
// These messages guide users to the new unified command structure.
var deprecationMessages = map[string]string{
	"init":      "ralph (auto-initializes) or ralph --parent <id>",
	"run":       "ralph",
	"decompose": "ralph <prd.md>",
	"import":    "ralph <tasks.yaml>",
	"pause":     "Ctrl+C to stop. Run ralph to resume",
	"resume":    "ralph",
	"retry":     "ralph fix --retry <id>",
	"skip":      "ralph fix --skip <id>",
	"revert":    "ralph fix --undo <id>",
	"logs":      "ralph status --log",
	"report":    "ralph status --report",
}

// warnDeprecated prints a deprecation warning to stderr for the given command.
// It returns true if the command is deprecated and execution should continue,
// or false if the command is not in the deprecation map.
func warnDeprecated(w io.Writer, command string) bool {
	replacement, ok := deprecationMessages[command]
	if !ok {
		return false
	}

	_, _ = fmt.Fprintf(w, "Deprecated: Use %s instead\n", replacement)
	return true
}
