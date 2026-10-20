package healthsummary

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	healthfake "github.com/harvester/resource-health-controller/pkg/generated/clientset/versioned/fake"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func newComponentHealth(name, component string, checks map[string]healthv1.CheckResult) *healthv1.ComponentHealth {
	health := &healthv1.ComponentHealth{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     healthv1.ComponentHealthStatus{Checks: checks},
	}
	if component != "" {
		health.Labels = map[string]string{healthv1.LabelKeyComponent: component}
	}
	return health
}

func TestReconcileSummaryGroupsByComponentLabel(t *testing.T) {
	clientset := healthfake.NewSimpleClientset(
		newComponentHealth("harvester-controller-node", "harvester-controller", map[string]healthv1.CheckResult{
			"NodeCordoned": {Severity: healthv1.SeverityInfo},
		}),
		newComponentHealth("harvester-controller-vm", "harvester-controller", map[string]healthv1.CheckResult{
			"VMHostDeviceLiveMigrationUnsupported": {Severity: healthv1.SeverityWarning},
			"SomethingBroken":                      {Severity: healthv1.SeverityError},
		}),
		newComponentHealth("harvester-controller-volume", "harvester-controller", map[string]healthv1.CheckResult{
			"ReplicaSchedulingFailure": {Severity: healthv1.SeverityWarning},
		}),
		newComponentHealth("other-component", "", map[string]healthv1.CheckResult{
			"Broken": {Severity: healthv1.SeverityError},
		}),
	)
	h := &Handler{
		healthSummaries:     fakeclients.HealthSummaryClient(clientset.HealthV1beta1().HealthSummaries),
		componentHealthCtrl: fakeclients.ComponentHealthClient(clientset.HealthV1beta1().ComponentHealths),
	}

	assert.NoError(t, h.reconcileSummary())

	summary, err := clientset.HealthV1beta1().HealthSummaries().Get(context.Background(), summaryName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, map[string]healthv1.ComponentSummary{
		// grouped by label: errors and warnings of every CR with the same label are summed
		"harvester-controller": {ErrorCount: 1, WarningCount: 2},
		// falls back to the CR name when the label is absent
		"other-component": {ErrorCount: 1},
	}, summary.Status.Components)
}

func TestReconcileSummaryDoesNotWriteWhenUnchanged(t *testing.T) {
	clientset := healthfake.NewSimpleClientset(
		newComponentHealth("harvester-controller-vm", "harvester-controller", map[string]healthv1.CheckResult{
			"Broken": {Severity: healthv1.SeverityError},
		}),
	)
	h := &Handler{
		healthSummaries:     fakeclients.HealthSummaryClient(clientset.HealthV1beta1().HealthSummaries),
		componentHealthCtrl: fakeclients.ComponentHealthClient(clientset.HealthV1beta1().ComponentHealths),
	}

	assert.NoError(t, h.reconcileSummary())
	assert.NoError(t, h.reconcileSummary())

	for _, action := range clientset.Actions() {
		assert.False(t, action.Matches("update", "healthsummaries"))
	}

	// a change to the checks updates the summary
	_, err := clientset.HealthV1beta1().ComponentHealths().Update(context.Background(),
		newComponentHealth("harvester-controller-vm", "harvester-controller", nil), metav1.UpdateOptions{})
	assert.NoError(t, err)
	assert.NoError(t, h.reconcileSummary())

	summary, err := clientset.HealthV1beta1().HealthSummaries().Get(context.Background(), summaryName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, map[string]healthv1.ComponentSummary{"harvester-controller": {}}, summary.Status.Components)
}

func TestOnComponentHealthChangedIgnoresDeletion(t *testing.T) {
	h := &Handler{}
	result, err := h.onComponentHealthChanged("harvester-controller-vm", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
}

func TestReconcileSummarySkipsIgnoredChecks(t *testing.T) {
	clientset := healthfake.NewSimpleClientset(
		newComponentHealth("harvester-controller-volume", "harvester-controller", map[string]healthv1.CheckResult{
			"ReplicaSchedulingFailure": {Severity: healthv1.SeverityWarning, Ignored: true, IgnoredBy: "rule"},
			"RestoreFailure":           {Severity: healthv1.SeverityWarning},
			"SomethingBroken":          {Severity: healthv1.SeverityError, Ignored: true, IgnoredBy: "rule"},
		}),
	)
	h := &Handler{
		healthSummaries:     fakeclients.HealthSummaryClient(clientset.HealthV1beta1().HealthSummaries),
		componentHealthCtrl: fakeclients.ComponentHealthClient(clientset.HealthV1beta1().ComponentHealths),
	}

	assert.NoError(t, h.reconcileSummary())

	summary, err := clientset.HealthV1beta1().HealthSummaries().Get(context.Background(), summaryName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, map[string]healthv1.ComponentSummary{
		"harvester-controller": {WarningCount: 1},
	}, summary.Status.Components)
}
