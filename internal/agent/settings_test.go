package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	for _, tc := range []struct{ agent, provider, effort string }{
		{"grok", "openai", ""}, {"cursor", "anthropic", ""},
		{"claude", "made-up", ""}, {"cursor", "", "high"},
	} {
		p, err := ByName(tc.agent)
		if err != nil {
			t.Fatal(err)
		}
		p.Provider, p.Effort = tc.provider, tc.effort
		if err := p.ValidateSettings(); err == nil {
			t.Fatalf("accepted unsupported settings: %+v", tc)
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
