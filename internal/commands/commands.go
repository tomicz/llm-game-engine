package commands

import (
	"flag"
	"fmt"
	"strings"
)

const prefix = "cmd "

// Command is a subcommand with its own flags and a Run function.
// Flags are defined on FlagSet; Run is called after Parse and can read flag state.
type Command struct {
	Name    string
	FlagSet *flag.FlagSet
	Run     func() error
}

// Registry holds subcommands by name. Add commands with Register; run with Execute.
type Registry struct {
	cmds map[string]*Command
}

// NewRegistry returns an empty command registry.
func NewRegistry() *Registry {
	return &Registry{cmds: make(map[string]*Command)}
}

// Register adds a subcommand. name is the first token after "cmd" (e.g. "grid").
// fs is that command's FlagSet; run is called after fs.Parse(args[1:]) succeeds.
func (r *Registry) Register(name string, fs *flag.FlagSet, run func() error) {
	r.cmds[name] = &Command{Name: name, FlagSet: fs, Run: run}
}

// Parse interprets line as a terminal line. If line starts with "cmd " (case-sensitive),
// the rest is tokenized by spaces and returned with ok true. Otherwise nil, false.
func Parse(line string) (args []string, ok bool) {
	if !strings.HasPrefix(line, prefix) {
		return nil, false
	}
	rest := strings.TrimSpace(line[len(prefix):])
	if rest == "" {
		return nil, true
	}
	return strings.Fields(rest), true
}

// Execute runs the subcommand in args[0] with args[1:] as flag/positional arguments.
// Returns an error for unknown command, parse error, or from Run().
func (r *Registry) Execute(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing subcommand")
	}
	name := args[0]
	cmd, ok := r.cmds[name]
	if !ok {
		return fmt.Errorf("unknown command: %s", name)
	}
	if err := cmd.FlagSet.Parse(args[1:]); err != nil {
		return err
	}
	return cmd.Run()
}
