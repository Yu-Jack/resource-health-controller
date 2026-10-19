package componenthealth

import (
	"context"
	"testing"
	"time"

	longhornv1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

const testCheckKey = "ReplicaSchedulingFailure"

func newIgnoreRule(name, checkKey string, resources []string, expiresAt *time.Time) *healthv1.HealthIgnoreRule {
	rule := &healthv1.HealthIgnoreRule{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: healthv1.HealthIgnoreRuleSpec{
			ComponentHealth: volumeComponentHealthName,
			CheckKey:        checkKey,
			Resources:       resources,
		},
	}
	if expiresAt != nil {
		rule.Spec.ExpiresAt = &metav1.Time{Time: *expiresAt}
	}
	return rule
}

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
			AffectedResources: &healthv1.AffectedResources{Kind: volumeKind, Names: details},
		},
	}
}

func TestApplyIgnoreRules(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	later := now.Add(2 * time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name           string
		rules          []*healthv1.HealthIgnoreRule
		minSeverity    healthv1.Severity
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
			rules:         []*healthv1.HealthIgnoreRule{newIgnoreRule("r", testCheckKey, nil, nil)},
			expectIgnored: true,
			expectedBy:    "r",
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:  "rule with resources only ignores those resources",
			rules: []*healthv1.HealthIgnoreRule{newIgnoreRule("r", testCheckKey, []string{"a"}, nil)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{
				"a": {Ignored: true, IgnoredBy: "r"},
				"b": {},
			},
		},
		{
			name: "check is ignored once every resource is ignored",
			rules: []*healthv1.HealthIgnoreRule{
				newIgnoreRule("r1", testCheckKey, []string{"a"}, nil),
				newIgnoreRule("r2", testCheckKey, []string{"b"}, nil),
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
			rules:         []*healthv1.HealthIgnoreRule{newIgnoreRule("r", "Other", nil, nil)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:          "rule naming an unknown resource does nothing",
			rules:         []*healthv1.HealthIgnoreRule{newIgnoreRule("r", testCheckKey, []string{"unknown"}, nil)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:          "expired rule does not apply",
			rules:         []*healthv1.HealthIgnoreRule{newIgnoreRule("r", testCheckKey, nil, &past)},
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name: "earliest expiry of the effective rules is reported",
			rules: []*healthv1.HealthIgnoreRule{
				newIgnoreRule("r1", testCheckKey, []string{"a"}, &later),
				newIgnoreRule("r2", testCheckKey, []string{"b"}, &future),
				newIgnoreRule("r3", testCheckKey, []string{"b"}, &past),
			},
			expectIgnored: true,
			expectedBy:    "r1",
			expectedNames: map[string]healthv1.AffectedResourceDetail{
				"a": {Ignored: true, IgnoredBy: "r1"},
				"b": {Ignored: true, IgnoredBy: "r2"},
			},
			expectedExpiry: &future,
		},
		{
			name:          "min severity ignores lower severity",
			minSeverity:   healthv1.SeverityError,
			expectIgnored: true,
			expectedBy:    healthv1.IgnoredByMinSeverity,
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
		{
			name:          "min severity keeps the same severity",
			minSeverity:   healthv1.SeverityWarning,
			expectedNames: map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checks := newTestChecks(healthv1.SeverityWarning, "a", "b")
			result, expiry := applyIgnoreRules(checks, tc.rules, tc.minSeverity, now)

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

	result, _ := applyIgnoreRules(checks, nil, "", time.Now())

	assert.Equal(t, newTestChecks(healthv1.SeverityWarning, "a"), result)
}

func volumeWithFalseCondition(name string) *longhornv1.Volume {
	return newLonghornVolume(name, []longhornv1.Condition{{
		Type:   longhornv1.VolumeConditionTypeScheduled,
		Status: longhornv1.ConditionStatusFalse,
		Reason: longhornv1.VolumeConditionReasonReplicaSchedulingFailure,
	}})
}

func TestReconcileVolumesAppliesIgnoreRules(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	clientset, healthset := newTestClients(
		volumeWithFalseCondition("pvc-aaa"),
		volumeWithFalseCondition("pvc-bbb"),
		newIgnoreRule("ignore-aaa", testCheckKey, []string{"pvc-aaa"}, &expiresAt),
		// targets another ComponentHealth
		&healthv1.HealthIgnoreRule{
			ObjectMeta: metav1.ObjectMeta{Name: "other"},
			Spec:       healthv1.HealthIgnoreRuleSpec{ComponentHealth: nodeComponentHealthName, CheckKey: testCheckKey},
		},
	)
	controller := &recordingComponentHealthController{}
	handler := &Handler{
		componentHealthController: controller,
		componentHealths:          fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
		volumeCache:               fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes),
		ignoreRuleCache:           fakeclients.HealthIgnoreRuleCache(healthset.HealthV1beta1().HealthIgnoreRules),
	}

	assert.NoError(t, handler.reconcileVolumes())

	componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	check := componentHealth.Status.Checks[testCheckKey]
	assert.False(t, check.Ignored)
	assert.Equal(t, 2, check.AffectedCount)
	assert.Equal(t, healthv1.AffectedResourceDetail{Ignored: true, IgnoredBy: "ignore-aaa"}, check.AffectedResources.Names["pvc-aaa"])
	assert.Equal(t, healthv1.AffectedResourceDetail{}, check.AffectedResources.Names["pvc-bbb"])
	// a reconcile is scheduled for the expiry
	assert.Equal(t, []string{volumeComponentHealthName}, controller.enqueuedAfter)

	// the rule is deleted: the mark goes away
	assert.NoError(t, healthset.HealthV1beta1().HealthIgnoreRules().Delete(context.Background(), "ignore-aaa", metav1.DeleteOptions{}))
	assert.NoError(t, handler.reconcileVolumes())
	componentHealth, err = healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, healthv1.AffectedResourceDetail{}, componentHealth.Status.Checks[testCheckKey].AffectedResources.Names["pvc-aaa"])
}

func TestReconcileVolumesIgnoreIsStable(t *testing.T) {
	clientset, healthset := newTestClients(
		volumeWithFalseCondition("pvc-aaa"),
		newIgnoreRule("ignore-all", testCheckKey, nil, nil),
	)
	handler := &Handler{
		componentHealths: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
		volumeCache:      fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes),
		ignoreRuleCache:  fakeclients.HealthIgnoreRuleCache(healthset.HealthV1beta1().HealthIgnoreRules),
	}

	assert.NoError(t, handler.reconcileVolumes())
	first, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.True(t, first.Status.Checks[testCheckKey].Ignored)

	// nothing changed: the second reconcile must not write again
	healthset.ClearActions()
	assert.NoError(t, handler.reconcileVolumes())
	for _, action := range healthset.Actions() {
		assert.NotEqual(t, "update", action.GetVerb())
	}
}

func TestOnIgnoreRuleChangedEnqueuesEveryComponentHealth(t *testing.T) {
	controller := &recordingComponentHealthController{}
	handler := &Handler{componentHealthController: controller}

	// a deleted rule is seen as nil
	result, err := handler.OnIgnoreRuleChanged("rule", nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Len(t, controller.enqueued, 6)
}
