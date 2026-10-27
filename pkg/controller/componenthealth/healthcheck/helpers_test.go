package healthcheck

import (
	clienttesting "k8s.io/client-go/testing"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

const (
	testCheckKey            = "ExampleFailure"
	testComponentHealthName = "harvester-controller-example"
)

func newTestChecks(severity healthv1.Severity, names ...string) map[string]healthv1.CheckResult {
	details := map[string]healthv1.AffectedResourceDetail{}
	for _, name := range names {
		details[name] = healthv1.AffectedResourceDetail{}
	}
	return map[string]healthv1.CheckResult{
		testCheckKey: {
			Severity:          severity,
			Message:           "message",
			AffectedCount:     len(names),
			AffectedResources: &healthv1.AffectedResources{Kind: "Example", Names: details},
		},
	}
}

func countActions(actions []clienttesting.Action, verb, resource string) int {
	count := 0
	for _, action := range actions {
		if action.Matches(verb, resource) {
			count++
		}
	}
	return count
}
