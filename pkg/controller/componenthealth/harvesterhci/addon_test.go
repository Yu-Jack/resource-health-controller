package harvesterhci

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func TestReconcileAddons(t *testing.T) {
	tests := []struct {
		name       string
		conditions []harvesterv1.Condition
		deleting   bool
		failed     bool
	}{
		{name: "new addon"},
		{name: "deleting addon is excluded", deleting: true, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.AddonOperationFailed, Status: corev1.ConditionTrue, Message: "job failed"},
		}},
		{name: "operation in progress", conditions: []harvesterv1.Condition{
			{Type: harvesterv1.AddonOperationInProgress, Status: corev1.ConditionTrue},
			{Type: harvesterv1.AddonOperationFailed, Status: corev1.ConditionFalse},
		}},
		{name: "operation completed", conditions: []harvesterv1.Condition{
			{Type: harvesterv1.AddonOperationCompleted, Status: corev1.ConditionTrue},
		}},
		{name: "operation failed", failed: true, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.AddonOperationFailed, Status: corev1.ConditionTrue, Reason: "JobFailed", Message: "helm install timed out"},
			{Type: harvesterv1.AddonOperationInProgress, Status: corev1.ConditionFalse},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addon := &harvesterv1.Addon{
				ObjectMeta: metav1.ObjectMeta{Name: "rancher-monitoring", Namespace: "cattle-monitoring-system"},
				Spec:       harvesterv1.AddonSpec{Enabled: true},
				Status:     harvesterv1.AddonStatus{Status: harvesterv1.AddonEnabling, Conditions: tc.conditions},
			}
			if tc.deleting {
				deletedAt := metav1.Now()
				addon.DeletionTimestamp = &deletedAt
			}
			clientset, healthset := testutil.NewClients(addon)
			writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			addons := &addonCheck{writer: writer, addonCache: fakeclients.AddonCache(clientset.HarvesterhciV1beta1().Addons)}
			if !assert.NoError(t, addons.Reconcile()) {
				return
			}
			health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), addonComponentHealthName, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			if !tc.failed {
				assert.Empty(t, health.Status.Checks)
				return
			}
			assert.Len(t, health.Status.Checks, 1)
			check := health.Status.Checks[checkKeyAddonOperationFailed]
			assert.Equal(t, "Error", string(check.Severity))
			assert.Equal(t, 1, check.AffectedCount)
			assert.Equal(t, addonKind, check.AffectedResources.Kind)
			assert.Equal(t, harvesterv1.SchemeGroupVersion.String(), check.AffectedResources.APIVersion)
			assert.Contains(t, check.AffectedResources.Names, "cattle-monitoring-system/rancher-monitoring")
			assert.Contains(t, check.Message, `state="AddonEnabling", enabled=true`)
			assert.Contains(t, check.Message, `reason="JobFailed", message="helm install timed out"`)
		})
	}
}

func TestAddonChecksClearAndPreserveNamespacedResources(t *testing.T) {
	ctx := context.Background()
	failedConditions := []harvesterv1.Condition{{Type: harvesterv1.AddonOperationFailed, Status: corev1.ConditionTrue, Message: "job failed"}}
	addon := &harvesterv1.Addon{
		ObjectMeta: metav1.ObjectMeta{Name: "vm-import-controller", Namespace: "harvester-system"},
		Status:     harvesterv1.AddonStatus{Conditions: failedConditions},
	}
	other := addon.DeepCopy()
	other.Name = "pcidevices-controller"
	clientset, healthset := testutil.NewClients(addon, other)
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller: controller,
		Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
	}
	addons := &addonCheck{writer: writer, addonCache: fakeclients.AddonCache(clientset.HarvesterhciV1beta1().Addons)}
	result, err := addons.OnAddonChanged("harvester-system/vm-import-controller", addon)
	assert.Same(t, addon, result)
	assert.NoError(t, err)
	assert.Equal(t, []string{addonComponentHealthName}, controller.Enqueued)
	if !assert.NoError(t, addons.Reconcile()) {
		return
	}
	health, err := healthset.HealthV1beta1().ComponentHealths().Get(ctx, addonComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	check := health.Status.Checks[checkKeyAddonOperationFailed]
	assert.Equal(t, 2, check.AffectedCount)

	// the operation is retried and succeeds
	addon.Status.Conditions = []harvesterv1.Condition{{Type: harvesterv1.AddonOperationFailed, Status: corev1.ConditionFalse}}
	if _, err := clientset.HarvesterhciV1beta1().Addons(addon.Namespace).Update(ctx, addon, metav1.UpdateOptions{}); !assert.NoError(t, err) {
		return
	}
	if !assert.NoError(t, addons.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, addonComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	check = health.Status.Checks[checkKeyAddonOperationFailed]
	assert.Equal(t, 1, check.AffectedCount)
	assert.NotContains(t, check.AffectedResources.Names, "harvester-system/vm-import-controller")

	if !assert.NoError(t, clientset.HarvesterhciV1beta1().Addons(other.Namespace).Delete(ctx, other.Name, metav1.DeleteOptions{})) {
		return
	}
	result, err = addons.OnAddonChanged("harvester-system/pcidevices-controller", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
	if !assert.NoError(t, addons.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, addonComponentHealthName, metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, health.Status.Checks)
	}
}

func TestReconcileAddonsListError(t *testing.T) {
	clientset, _ := testutil.NewClients()
	clientset.PrependReactor("list", "addons", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("cache failure")
	})
	addons := &addonCheck{addonCache: fakeclients.AddonCache(clientset.HarvesterhciV1beta1().Addons)}
	assert.EqualError(t, addons.Reconcile(), "failed to list Addons: cache failure")
}
