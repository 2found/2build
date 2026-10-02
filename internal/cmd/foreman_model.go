package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/reallongnguyen/babysit/internal/config"
)

// foremanModel is an offline policy lookup, not a launch or capability check.
func foremanModel(args []string) error {
	id, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	if id != "" {
		return fmt.Errorf("foreman model: unexpected argument %q", id)
	}
	for key := range kv {
		switch key {
		case "agent", "complexity", "phase-class", "dir", "json":
		default:
			return fmt.Errorf("foreman model: unknown flag --%s", key)
		}
	}
	dir := kv["dir"]
	if dir == "" {
		dir, err = os.Getwd()
	} else {
		dir, err = filepath.Abs(dir)
	}
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("foreman model: --dir must be a directory")
	}
	if root := gitOutIn(dir, "rev-parse", "--show-toplevel"); root != "" {
		dir = root
	}
	policy, err := config.LoadForemanModels(dir)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if kv["agent"] == "" && kv["complexity"] == "" && kv["phase-class"] == "" {
		return encoder.Encode(policy)
	}
	for _, key := range []string{"agent", "complexity", "phase-class"} {
		if strings.TrimSpace(kv[key]) == "" {
			return fmt.Errorf("foreman model: --%s is required for a model lookup", key)
		}
	}
	agent := strings.ToLower(strings.TrimSpace(kv["agent"]))
	complexity := kv["complexity"]
	if complexity == "critical" {
		complexity = "hard"
	}
	tier, binding, err := policy.Select(agent, complexity, kv["phase-class"])
	if err != nil {
		return err
	}
	return encoder.Encode(struct {
		Agent        string `json:"agent"`
		Complexity   string `json:"complexity"`
		PhaseClass   string `json:"phaseClass"`
		SelectedTier string `json:"selectedTier"`
		config.ModelBinding
	}{agent, complexity, kv["phase-class"], tier, binding})
}
