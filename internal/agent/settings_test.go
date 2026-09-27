package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsPrecedenceAndRoleIndependence(t *testing.T) {
	isolate(t)
	writeGlobal(t, "worker_agent: omp\nworker_provider: global\nworker_model: global-model\nworker_effort: low\nforeman_model: audit-model\n")
	check := func(opts Options, provider, model, effort string) {
		t.Helper()
		p, err := ResolveWith(WorkerKey, opts)
		if err != nil || p.Name != "omp" || p.Provider != provider || p.Model != model || p.Effort != effort {
			t.Fatalf("resolved %+v, err %v; want omp/%s/%s/%s", p, err, provider, model, effort)
		}
	}
	check(Options{}, "global", "global-model", "low")
	t.Setenv("BABYSIT_PROVIDER", "env")
	t.Setenv("BABYSIT_MODEL", "env-model")
	t.Setenv("BABYSIT_EFFORT", "high")
	check(Options{}, "env", "env-model", "high")
	t.Setenv("BABYSIT_WORKER_PROVIDER", "role")
	t.Setenv("BABYSIT_WORKER_MODEL", "role-model")
	check(Options{}, "role", "role-model", "high")
	check(Options{Provider: "flag", Model: "flag-model", Effort: "max"}, "flag", "flag-model", "max")
	t.Setenv("BABYSIT_PROVIDER", "")
	t.Setenv("BABYSIT_MODEL", "")
	t.Setenv("BABYSIT_EFFORT", "")
	foreman, err := Resolve(ForemanKey, "")
	if err != nil || foreman.Name != "claude" || foreman.Model != "audit-model" || foreman.Provider != "" || foreman.Effort != "" {
		t.Fatalf("worker settings leaked into foreman: %+v, %v", foreman, err)
	}
}

func TestLaunchSettingsUseNativeFlags(t *testing.T) {
	for _, tc := range []struct{ name, provider, effort, want string }{
		{"omp", "my-provider", "high", "omp --auto-approve --provider 'my-provider' --model 'future-model' --thinking 'high' 'prompt'"},
		{"codex", "custom", "high", `codex --dangerously-bypass-approvals-and-sandbox -c 'model_provider="custom"' --model 'future-model' -c 'model_reasoning_effort="high"' 'prompt'`},
		{"grok", "xai", "high", "grok --always-approve --model 'future-model' --reasoning-effort 'high' 'prompt'"},
		{"cursor", "cursor", "", "cursor-agent --yolo --model 'future-model' 'prompt'"},
		{"claude", "bedrock", "high", "CLAUDE_CODE_USE_FOUNDRY=0 CLAUDE_CODE_USE_VERTEX=0 CLAUDE_CODE_USE_BEDROCK=1 claude --dangerously-skip-permissions --model 'future-model' --effort 'high' 'prompt'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := ByName(tc.name)
			p.Provider, p.Model, p.Effort = tc.provider, "future-model", tc.effort
			if err := p.ValidateSettings(); err != nil {
				t.Fatal(err)
			}
			if got := p.WorkerCommand("prompt"); got != tc.want {
				t.Fatalf("command %q, want %q", got, tc.want)
			}
			for _, cmd := range []string{p.NewSessionCommand("session", "prompt"), p.ResumeCommand("session", "prompt")} {
				if !strings.Contains(cmd, "--model 'future-model'") {
					t.Fatalf("session command dropped preferences: %s", cmd)
				}
			}
		})
	}
}

func TestUnsupportedSettingsFailBeforeLaunch(t *testing.T) {
	isolate(t)
	for _, opts := range []Options{
		{Agent: "grok", Provider: "openai"}, {Agent: "cursor", Provider: "anthropic"},
		{Agent: "claude", Provider: "made-up"}, {Agent: "cursor", Effort: "high"},
	} {
		if _, err := ResolveWith(WorkerKey, opts); err == nil {
			t.Fatalf("accepted unsupported settings: %+v", opts)
		}
	}
}

func TestLaunchValuesRemainLiteralShellArguments(t *testing.T) {
	isolate(t)
	bin := filepath.Join(os.Getenv("PATH"), "omp")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, _ := ByName("omp")
	p.Model = "provider/model'; $(touch should-not-exist) `echo injected`"
	p.Provider = "a' provider"
	out, err := exec.Command("/bin/sh", "-c", p.WorkerCommand("don't expand $HOME")).CombinedOutput()
	if err != nil {
		t.Fatalf("shell: %v %s", err, out)
	}
	want := "--auto-approve\n--provider\n" + p.Provider + "\n--model\n" + p.Model + "\ndon't expand $HOME\n"
	if string(out) != want {
		t.Fatalf("arguments changed: %q, want %q", out, want)
	}
}

func TestUnspecifiedModelDoesNotOverrideNativeConfig(t *testing.T) {
	isolate(t)
	for _, name := range Names() {
		p, err := Resolve(WorkerKey, name)
		if err != nil || p.Provider != "" || p.Model != "" || p.Effort != "" {
			t.Fatalf("fabricated preferences for %s: %+v %v", name, p, err)
		}
	}
}
