package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ConfirmScope says which service links raise the leaving dialog.
//
// A bool would have been enough to turn the dialog on, and not enough to say
// what most operators actually want. cairn already knows, per service, whether
// the host runs the thing or merely points at it, and that is the line the
// warning belongs on: a visitor being sent to somebody else's site is the case
// worth a sentence, while being sent to the host's own Nextcloud is not. So
// the key takes that answer rather than a yes.
//
// `true` is accepted and means `all`, because a reader who has seen new_tab
// above will write it and should not be met with a type error.
type ConfirmScope string

const (
	ConfirmOff      ConfirmScope = ""
	ConfirmAll      ConfirmScope = "all"
	ConfirmExternal ConfirmScope = "external"
)

func (c *ConfirmScope) UnmarshalYAML(n *yaml.Node) error {
	var raw string
	if err := n.Decode(&raw); err != nil {
		return fmt.Errorf("line %d: expected all, external, or true/false", n.Line)
	}
	switch raw {
	case "true", "all":
		*c = ConfirmAll
	case "false", "off", "":
		*c = ConfirmOff
	case "external":
		*c = ConfirmExternal
	default:
		return fmt.Errorf("line %d: %q is not a scope; write all for every link that leaves the site, or external for the services flagged external", n.Line, raw)
	}
	return nil
}

// Wants reports whether a service raises the dialog. hostKind is the card's
// own flag: "self", "external", or empty when the config says nothing.
//
// Under `external`, a service with no flag at all does not qualify. That is
// the literal reading and the safe one: cairn refuses to guess that silence
// means somebody else's site, when silence is what a config says before the
// operator has thought about it.
func (c ConfirmScope) Wants(hostKind string) bool {
	switch c {
	case ConfirmAll:
		return true
	case ConfirmExternal:
		return hostKind == "external"
	default:
		return false
	}
}
