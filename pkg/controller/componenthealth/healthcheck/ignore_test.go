package healthcheck

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func newTestIgnoreRule(name, checkKey string, resources []string, expiresAt *time.Time) *healthv1.HealthIgnoreRule {
	return testutil.NewIgnoreRule(testComponentHealthName, name, checkKey, resources, expiresAt)
}

func TestApplyIgnoreRules(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	later := now.Add(2 * time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name           string
		rules          []*healthv1.HealthIgnoreRule
		expectIgnored  bool
		expectedBy     string
		expectedNames  map[string]healthv1.AffectedResourceDetail
		expectedExpiry *time.Time
	}{
		{
			name:          "no rule",
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:          "rule without resources ignores the whole check",
			rules:         []*healthv1.HealthIgnoreRule{newTestIgnoreRule("r", testCheckKey, nil, nil)},
			expectIgnored: true,
			expectedBy:    "r",
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:  "rule with resources only ignores those resources",
			rules: []*healthv1.HealthIgnoreRule{newTestIgnoreRule("r", testCheckKey, []string{"a"}, nil)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{
				"a": {Ignored: true, IgnoredBy: "r"},
				"b": {},
			},
		},
		{
			name: "check is ignored once every resource is ignored",
			rules: []*healthv1.HealthIgnoreRule{
				newTestIgnoreRule("r1", testCheckKey, []string{"a"}, nil),
				newTestIgnoreRule("r2", testCheckKey, []string{"b"}, nil),
			},
			expectIgnored: true,
			expectedBy:    "r1",
			expectedNames: map[string]healthv1.AffectedResourceDetail{
				"a": {Ignored: true, IgnoredBy: "r1"},
				"b": {Ignored: true, IgnoredBy: "r2"},
			},
		},
		{
			name:          "rule of another check does not apply",
			rules:         []*healthv1.HealthIgnoreRule{newTestIgnoreRule("r", "Other", nil, nil)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:          "rule naming an unknown resource does nothing",
			rules:         []*healthv1.HealthIgnoreRule{newTestIgnoreRule("r", testCheckKey, []string{"unknown"}, nil)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:          "expired rule does not apply",
			rules:         []*healthv1.HealthIgnoreRule{newTestIgnoreRule("r", testCheckKey, nil, &past)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name: "earliest expiry of the effective rules is reported",
			rules: []*healthv1.HealthIgnoreRule{
				newTestIgnoreRule("r1", testCheckKey, []string{"a"}, &later),
				newTestIgnoreRule("r2", testCheckKey, []string{"b"}, &future),
				newTestIgnoreRule("r3", testCheckKey, []string{"b"}, &past),
			},
			expectIgnored: true,
			expectedBy:    "r1",
			expectedNames: map[string]healthv1.AffectedResourceDetail{
				"a": {Ignored: true, IgnoredBy: "r1"},
				"b": {Ignored: true, IgnoredBy: "r2"},
			},
			expectedExpiry: &future,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checks := newTestChecks(healthv1.SeverityWarning, "a", "b")
			result, expiry := applyIgnoreRules(checks, tc.rules, now)

			check := result[testCheckKey]
			assert.Equal(t, tc.expectIgnored, check.Ignored)
			assert.Equal(t, tc.expectedBy, check.IgnoredBy)
			assert.Equal(t, tc.expectedNames, check.AffectedResources.Names)
			// the raw facts are kept
			assert.Equal(t, 2, check.AffectedCount)
			if tc.expectedExpiry == nil {
				assert.Nil(t, expiry)
			} else if assert.NotNil(t, expiry) {
				assert.True(t, tc.expectedExpiry.Equal(*expiry))
			}
			// the input is never mutated
			assert.Equal(t, newTestChecks(healthv1.SeverityWarning, "a", "b"), checks)
		})
	}
}

func TestApplyIgnoreRulesClearsStaleMarks(t *testing.T) {
	checks := newTestChecks(healthv1.SeverityWarning, "a")
	stale := checks[testCheckKey]
	stale.Ignored = true
	stale.IgnoredBy = "deleted-rule"
	stale.AffectedResources.Names["a"] = healthv1.AffectedResourceDetail{Ignored: true, IgnoredBy: "deleted-rule"}
	checks[testCheckKey] = stale

	result, _ := applyIgnoreRules(checks, nil, time.Now())

	assert.Equal(t, newTestChecks(healthv1.SeverityWarning, "a"), result)
}

func TestListIgnoreRules(t *testing.T) {
	_, healthset := testutil.NewClients(
		newTestIgnoreRule("rule-b", testCheckKey, nil, nil),
		newTestIgnoreRule("rule-a", testCheckKey, nil, nil),
		&healthv1.HealthIgnoreRule{
			ObjectMeta: metav1.ObjectMeta{Name: "other"},
			Spec:       healthv1.HealthIgnoreRuleSpec{ComponentHealth: "harvester-controller-other", CheckKey: testCheckKey},
		},
	)

	rules, err := listIgnoreRules(fakeclients.HealthIgnoreRuleCache(healthset.HealthV1beta1().HealthIgnoreRules), testComponentHealthName)

	assert.NoError(t, err)
	names := make([]string, 0, len(rules))
	for _, rule := range rules {
		names = append(names, rule.Name)
	}
	assert.Equal(t, []string{"rule-a", "rule-b"}, names)
}

func TestListIgnoreRulesWithoutCache(t *testing.T) {
	rules, err := listIgnoreRules(nil, testComponentHealthName)

	assert.NoError(t, err)
	assert.Empty(t, rules)
}
