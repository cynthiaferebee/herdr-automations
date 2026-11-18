package host

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/DnzzL/herdr-automations/internal/config"
)

// modelRoute resolves a routing job to the launch id model-route picks right
// now, "<kind>/<model>". A var so the tests can route without the machine's
// routing table.
var modelRoute = func(job string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "model-route", job).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("model-route %s: %s", job, strings.TrimSpace(string(exit.Stderr)))
		}
		return "", fmt.Errorf("model-route %s: %w", job, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ResolveRoute returns the automation with its routing job resolved: the
// agent kind and model model-route picks right now replace route, agent and
// model. The runner resolves before provisioning, so a job that does not
// resolve fails the run without leaving a workspace behind.
func ResolveRoute(a config.Automation) (config.Automation, error) {
	kind, model, err := resolveRoute(a.Route)
	if err != nil {
		return a, fmt.Errorf("%s: route %q did not resolve: %w", a.Name, a.Route, err)
	}
	log.Printf("%s: route %q resolved to %s --model %q", a.Name, a.Route, kind, model)
	a.Agent, a.Model = kind, model
	return a, nil
}

// resolveRoute turns an automation's routing job into the agent kind and
// model the run starts with. It runs at dispatch, not at load: which model a
// job names is a fact of that moment — quotas exhaust and subscriptions come
// and go — and resolving early would pin the run to a stale answer.
func resolveRoute(job string) (kind, model string, err error) {
	id, err := modelRoute(job)
	if err != nil {
		return "", "", err
	}
	kind, model, _ = strings.Cut(id, "/")
	if kind == "" {
		return "", "", fmt.Errorf("model-route %s returned %q, want <kind>/<model>", job, id)
	}
	return kind, model, nil
}
