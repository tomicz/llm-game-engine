// Package commands implements the in-game terminal command system: lines of the form
// "cmd <name> [args...]" run a registered Command.
package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

const prefix = "cmd "

// Command is a terminal subcommand.
type Command struct {
	Name  string
	Usage string // argument synopsis, e.g. "--show | --hide"; empty for none
	Help  string // one-line description, shown by "cmd help" and given to the AI agent
	// Manual commands can only be typed by the user; the AI agent may not run them.
	Manual bool
	// Run receives the arguments after the command name, unparsed. Commands with flags parse
	// them themselves (see ParseToggle and NewFlagSet), so positional values like "-9.8" work.
	Run func(args []string) error
}

// Registry holds commands in registration order.
type Registry struct {
	cmds  map[string]Command
	order []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{cmds: make(map[string]Command)}
}

// Register adds c. It panics if the name is empty or already registered (a programming error).
func (r *Registry) Register(c Command) {
	if c.Name == "" || c.Run == nil {
		panic("commands: command needs a name and a Run func")
	}
	if _, dup := r.cmds[c.Name]; dup {
		panic("commands: duplicate command " + c.Name)
	}
	r.cmds[c.Name] = c
	r.order = append(r.order, c.Name)
}

// Lookup returns the command with the given name.
func (r *Registry) Lookup(name string) (Command, bool) {
	c, ok := r.cmds[name]
	return c, ok
}

// Commands returns all commands in registration order.
func (r *Registry) Commands() []Command {
	out := make([]Command, len(r.order))
	for i, name := range r.order {
		out[i] = r.cmds[name]
	}
	return out
}

// Parse interprets a terminal line. If it starts with "cmd " (case-sensitive), the rest is split
// on whitespace and returned with ok true. Otherwise it returns nil, false.
func Parse(line string) (args []string, ok bool) {
	rest, ok := strings.CutPrefix(line, prefix)
	if !ok {
		return nil, false
	}
	return strings.Fields(rest), true
}

// Execute runs the command named by args[0] with the remaining args.
func (r *Registry) Execute(args []string) error {
	if len(args) == 0 {
		return errors.New("missing subcommand (try cmd help)")
	}
	c, ok := r.cmds[args[0]]
	if !ok {
		return fmt.Errorf("unknown command: %s (try cmd help)", args[0])
	}
	return c.Run(args[1:])
}

// Synopsis is "name usage", e.g. "grid --show | --hide".
func (c Command) Synopsis() string {
	return strings.TrimSpace(c.Name + " " + c.Usage)
}

// UsageError returns an error showing the command's usage.
func (c Command) UsageError() error {
	return fmt.Errorf("usage: cmd %s", c.Synopsis())
}

// NewFlagSet returns a flag set for a command that reports errors instead of printing them.
func NewFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// ParseToggle parses a pair of boolean flags such as --show/--hide. set is false when neither
// was given; giving both is an error.
func ParseToggle(name string, args []string, on, off string) (value, set bool, err error) {
	fs := NewFlagSet(name)
	onV := fs.Bool(on, false, "")
	offV := fs.Bool(off, false, "")
	if err := fs.Parse(args); err != nil {
		return false, false, err
	}
	if *onV && *offV {
		return false, false, fmt.Errorf("use either --%s or --%s, not both", on, off)
	}
	return *onV, *onV || *offV, nil
}
