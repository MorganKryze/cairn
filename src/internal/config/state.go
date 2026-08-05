package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// State is where a service stands, as the operator declares it. It is not the
// status: the status is what a monitor measured a moment ago and changes on
// its own, while this changes when somebody edits a file. The two sit side by
// side on a card.
type State string

const (
	StateNone       State = ""
	StateSoon       State = "soon"
	StateRetired    State = "retired"
	StateBeta       State = "beta"
	StateDeprecated State = "deprecated"
	StateNew        State = "new"
)

// states is the closed set, in the order the refusal prints them: the two that
// disable come first, being the ones that change more than a label.
var states = []State{StateSoon, StateRetired, StateBeta, StateDeprecated, StateNew}

// Disables reports whether the state takes the destination away. Those two are
// the only ones that touch anything beyond the badge: no link, no status slot,
// no Open button, and a muted card.
func (s State) Disables() bool { return s == StateSoon || s == StateRetired }

func (s *State) UnmarshalYAML(n *yaml.Node) error {
	var v string
	if err := n.Decode(&v); err != nil {
		return err
	}
	for _, k := range states {
		if v == string(k) {
			*s = k
			return nil
		}
	}
	return fmt.Errorf("unknown state %q (expected one of: %s)", v, statesList())
}

func statesList() string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = string(s)
	}
	return strings.Join(out, ", ")
}
