package crawler

import (
	"encoding/json"
	"strings"
)

type npmCommand struct {
	field  []string
	target string
}

// commandEntry interprets npm's executable string or command-to-file map. Empty
// declarations install no command; unsupported shapes cannot prove absence.
// https://docs.npmjs.com/cli/v11/configuring-npm/package-json/#bin
func (p *packageJSON) commandEntry() (npmCommand, tristate) {
	if len(p.Bin) == 0 || string(p.Bin) == "null" {
		return npmCommand{}, no
	}
	var target string
	if err := json.Unmarshal(p.Bin, &target); err == nil {
		if target == "" {
			return npmCommand{}, no
		}
		if strings.TrimSpace(target) == "" {
			return npmCommand{}, unknown
		}
		return npmCommand{field: []string{"bin"}, target: target}, yes
	}
	var commands map[string]string
	if err := json.Unmarshal(p.Bin, &commands); err != nil {
		return npmCommand{}, unknown
	}
	if len(commands) == 0 {
		return npmCommand{}, no
	}
	names := sortedKeys(commands)
	for _, name := range names {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(commands[name]) == "" {
			return npmCommand{}, unknown
		}
	}
	name := names[0]
	return npmCommand{field: []string{"bin", name}, target: commands[name]}, yes
}
