package cmd

import (
	"fmt"
	"strings"

	"github.com/2found/2build/internal/agent"
	"github.com/2found/2build/internal/foreman"
)

const (
	planWorkerFlag      = "plan-worker"
	implementWorkerFlag = "implement-worker"
	qaWorkerFlag        = "qa-worker"
)

func parseForemanWorker(raw, flag string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	name := agent.Normalize(raw)
	if _, err := agent.ByName(name); err != nil {
		return "", fmt.Errorf("foreman: --%s must be one of %s", flag, strings.Join(agent.Names(), ", "))
	}
	return name, nil
}

func applyForemanWorkers(r *foreman.Record, kv map[string]string) error {
	for _, flag := range []string{planWorkerFlag, implementWorkerFlag, qaWorkerFlag} {
		name, err := parseForemanWorker(kv[flag], flag)
		if err != nil {
			return err
		}
		if name == "" {
			continue
		}
		switch flag {
		case planWorkerFlag:
			r.PlanWorker = name
		case implementWorkerFlag:
			r.ImplementWorker = name
		case qaWorkerFlag:
			r.QAWorker = name
		}
	}
	return nil
}

func resolvedForemanWorker(r foreman.Record, recorded string) string {
	if recorded != "" {
		return recorded
	}
	if r.Agent != "" {
		return agent.Normalize(r.Agent)
	}
	return agent.Default
}

func formatForemanWorkers(r foreman.Record) string {
	return fmt.Sprintf("workers plan=%s implement=%s qa=%s",
		resolvedForemanWorker(r, r.PlanWorker),
		resolvedForemanWorker(r, r.ImplementWorker),
		resolvedForemanWorker(r, r.QAWorker))
}

func foremanWorkerPromptFlags(r foreman.Record) string {
	var b strings.Builder
	for _, role := range []struct{ flag, value string }{
		{planWorkerFlag, r.PlanWorker},
		{implementWorkerFlag, r.ImplementWorker},
		{qaWorkerFlag, r.QAWorker},
	} {
		if role.value != "" {
			b.WriteString(" --")
			b.WriteString(role.flag)
			b.WriteByte(' ')
			b.WriteString(role.value)
		}
	}
	return b.String()
}
