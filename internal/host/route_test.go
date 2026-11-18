package host

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DnzzL/herdr-automations/internal/config"
)

func withModelRoute(t *testing.T, f func(string) (string, error)) {
	t.Helper()
	keep := modelRoute
	modelRoute = f
	t.Cleanup(func() { modelRoute = keep })
}

// The route resolves to the agent kind and --model the run starts with,
// straight from model-route's answer.
func TestResolveRouteSetsKindAndModel(t *testing.T) {
	withModelRoute(t, func(job string) (string, error) {
		if job != "coding" {
			t.Fatalf("model-route got job %q, want coding", job)
		}
		return "pi/anthropic/claude-haiku-5-5", nil
	})
	a, err := ResolveRoute(config.Automation{Name: "nightly", Route: "coding", Prompt: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Agent != "pi" || a.Model != "anthropic/claude-haiku-5-5" {
		t.Fatalf("got %s --model %q, want pi --model anthropic/claude-haiku-5-5", a.Agent, a.Model)
	}
}

// A job that does not resolve keeps model-route's reason in the error: that
// is the line the run is recorded with.
func TestResolveRouteKeepsTheResolversReason(t *testing.T) {
	withModelRoute(t, func(string) (string, error) {
		return "", errors.New("unknown job 'codding'; jobs: coding, writing")
	})
	_, err := ResolveRoute(config.Automation{Name: "nightly", Route: "codding", Prompt: "p"})
	if err == nil || !strings.Contains(err.Error(), "unknown job 'codding'") {
		t.Fatalf("want the resolver's reason, got %v", err)
	}
}

// A resolved run behaves like one with a static agent and model: the kind is
// started with --model, and a refused start falls back down fallback_models.
func TestResolvedRunFallsBackLikeAStaticModel(t *testing.T) {
	withModelRoute(t, func(string) (string, error) {
		return "pi/kimi-coding/k3", nil
	})
	a, err := ResolveRoute(config.Automation{
		Name: "nightly", Route: "coding", Prompt: "p",
		FallbackModels: []string{"anthropic/claude-haiku-5-5"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var kind string
	var models []string
	ops := &fakeOps{
		agentStart: func(_, k, _ string, args []string) error {
			kind = k
			for i, arg := range args {
				if arg == "--model" && i+1 < len(args) {
					models = append(models, args[i+1])
				}
			}
			if len(models) == 1 {
				return errors.New("model not found")
			}
			return nil
		},
		agentSubmit: func(string, string) error { return nil },
		agentWait:   func(string, time.Duration) error { return nil },
	}
	if err := agentWorkWith(ops, a).do(Session{PaneID: "p"}, time.Minute); err != nil {
		t.Fatalf("want the fallback to carry the routed run, got %v", err)
	}
	if kind != "pi" {
		t.Fatalf("started kind %q, want pi", kind)
	}
	if len(models) != 2 || models[0] != "kimi-coding/k3" || models[1] != "anthropic/claude-haiku-5-5" {
		t.Fatalf("models tried %q, want [kimi-coding/k3 anthropic/claude-haiku-5-5]", models)
	}
}
