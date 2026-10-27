package componenthealth

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func countActions(actions []clienttesting.Action, verb, resource string) int {
	count := 0
	for _, action := range actions {
		if action.Matches(verb, resource) {
			count++
		}
	}
	return count
}

func TestReplaceComponentHealthChecksCreatesWithIgnoreMarksInOneWrite(t *testing.T) {
	_, healthset := newTestClients(newIgnoreRule("ignore-all", testCheckKey, nil, nil))
	writer := componentHealthWriter{
		client:      fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
		ignoreRules: fakeclients.HealthIgnoreRuleCache(healthset.HealthV1beta1().HealthIgnoreRules),
	}

	err := writer.replaceComponentHealthChecks(volumeComponentHealthName, newTestChecks(healthv1.SeverityWarning, "a"))
	assert.NoError(t, err)

	health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, componentName, health.Labels[healthv1.LabelKeyComponent])
	check := health.Status.Checks[testCheckKey]
	assert.True(t, check.Ignored)
	assert.Equal(t, "ignore-all", check.IgnoredBy)
	assert.Equal(t, 1, countActions(healthset.Actions(), "create", "componenthealths"))
	assert.Equal(t, 0, countActions(healthset.Actions(), "update", "componenthealths"))
}

func TestReplaceComponentHealthChecksSkipsUnchangedChecks(t *testing.T) {
	_, healthset := newTestClients()
	writer := componentHealthWriter{client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
	checks := newTestChecks(healthv1.SeverityWarning, "a")

	assert.NoError(t, writer.replaceComponentHealthChecks(volumeComponentHealthName, checks))
	assert.NoError(t, writer.replaceComponentHealthChecks(volumeComponentHealthName, checks))

	assert.Equal(t, 1, countActions(healthset.Actions(), "create", "componenthealths"))
	assert.Equal(t, 0, countActions(healthset.Actions(), "update", "componenthealths"))
}

func TestReplaceComponentHealthChecksReplacesExistingChecks(t *testing.T) {
	_, healthset := newTestClients(&healthv1.ComponentHealth{
		ObjectMeta: metav1.ObjectMeta{Name: volumeComponentHealthName},
		Status: healthv1.ComponentHealthStatus{
			Checks: map[string]healthv1.CheckResult{
				"StaleCheck": {Severity: healthv1.SeverityError, Message: "stale"},
			},
		},
	})
	writer := componentHealthWriter{client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}

	err := writer.replaceComponentHealthChecks(volumeComponentHealthName, newTestChecks(healthv1.SeverityWarning, "a"))
	assert.NoError(t, err)

	health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	assert.NotContains(t, health.Status.Checks, "StaleCheck")
	assert.Contains(t, health.Status.Checks, testCheckKey)
	assert.Equal(t, 1, countActions(healthset.Actions(), "update", "componenthealths"))
	assert.Equal(t, componentName, health.Labels[healthv1.LabelKeyComponent])
}

func TestReplaceComponentHealthChecksErrors(t *testing.T) {
	failOn := func(verb, resource string) func(*testing.T) error {
		return func(t *testing.T) error {
			_, healthset := newTestClients()
			healthset.PrependReactor(verb, resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("boom")
			})
			writer := componentHealthWriter{
				client:      fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
				ignoreRules: fakeclients.HealthIgnoreRuleCache(healthset.HealthV1beta1().HealthIgnoreRules),
			}
			return writer.replaceComponentHealthChecks(volumeComponentHealthName, newTestChecks(healthv1.SeverityWarning, "a"))
		}
	}

	tests := []struct {
		name     string
		run      func(*testing.T) error
		expected string
	}{
		{
			name:     "listing ignore rules fails",
			run:      failOn("list", "healthignorerules"),
			expected: "failed to list HealthIgnoreRules: boom",
		},
		{
			name:     "getting the ComponentHealth fails",
			run:      failOn("get", "componenthealths"),
			expected: "failed to get ComponentHealth " + volumeComponentHealthName + ": boom",
		},
		{
			name:     "creating the ComponentHealth fails",
			run:      failOn("create", "componenthealths"),
			expected: "failed to create ComponentHealth " + volumeComponentHealthName + ": boom",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.EqualError(t, tc.run(t), tc.expected)
		})
	}
}
