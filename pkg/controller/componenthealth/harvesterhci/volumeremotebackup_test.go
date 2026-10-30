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
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func TestReconcileVolumeRemoteBackups(t *testing.T) {
	tests := []struct {
		name       string
		conditions []harvesterv1.Condition
		deleting   bool
		failed     bool
	}{
		{name: "new object"},
		{name: "deleting object is excluded", deleting: true, conditions: []harvesterv1.Condition{
			{Type: volumeRemoteReady, Status: corev1.ConditionFalse, Message: "backup target is unreachable"},
		}},
		{name: "operation in progress", conditions: []harvesterv1.Condition{
			{Type: volumeRemoteReady, Status: corev1.ConditionFalse, Message: volumeRemoteInProgressMessage},
		}},
		{name: "operation complete", conditions: []harvesterv1.Condition{
			{Type: volumeRemoteReady, Status: corev1.ConditionTrue, Message: "Operation complete"},
		}},
		{name: "operation failed", failed: true, conditions: []harvesterv1.Condition{
			{Type: volumeRemoteReady, Status: corev1.ConditionFalse, Message: "backup target is unreachable"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := &harvesterv1.VolumeRemoteBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "obj-a", Namespace: "default"},
				Status:     harvesterv1.VolumeRemoteBackupStatus{Conditions: tc.conditions},
			}
			if tc.deleting {
				deletedAt := metav1.Now()
				obj.DeletionTimestamp = &deletedAt
			}
			clientset, healthset := testutil.NewClients(obj)
			writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			check := &volumeRemoteBackupCheck{writer: writer, cache: fakeclients.VolumeRemoteBackupCache(clientset.HarvesterhciV1beta1().VolumeRemoteBackups)}
			if !assert.NoError(t, check.Reconcile()) {
				return
			}
			health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeRemoteBackupComponentHealthName, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			if !tc.failed {
				assert.Empty(t, health.Status.Checks)
				return
			}
			assert.Len(t, health.Status.Checks, 1)
			result := health.Status.Checks[checkKeyVolumeRemoteBackupFailed]
			assert.Equal(t, healthv1.SeverityError, result.Severity)
			assert.Equal(t, 1, result.AffectedCount)
			assert.Equal(t, volumeRemoteBackupKind, result.AffectedResources.Kind)
			assert.Equal(t, harvesterv1.SchemeGroupVersion.String(), result.AffectedResources.APIVersion)
			assert.Contains(t, result.AffectedResources.Names, "default/obj-a")
			assert.Contains(t, result.Message, `message="backup target is unreachable"`)
		})
	}
}

func TestVolumeRemoteBackupChecksClearAndPreserveNamespacedResources(t *testing.T) {
	ctx := context.Background()
	failed := []harvesterv1.Condition{{Type: volumeRemoteReady, Status: corev1.ConditionFalse, Message: "failed"}}
	obj := &harvesterv1.VolumeRemoteBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "obj-a", Namespace: "default"},
		Status:     harvesterv1.VolumeRemoteBackupStatus{Conditions: failed},
	}
	other := obj.DeepCopy()
	other.Name = "obj-b"
	clientset, healthset := testutil.NewClients(obj, other)
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller: controller,
		Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
	}
	check := &volumeRemoteBackupCheck{writer: writer, cache: fakeclients.VolumeRemoteBackupCache(clientset.HarvesterhciV1beta1().VolumeRemoteBackups)}
	result, err := check.OnVolumeRemoteBackupChanged("default/obj-a", obj)
	assert.Same(t, obj, result)
	assert.NoError(t, err)
	assert.Equal(t, []string{volumeRemoteBackupComponentHealthName}, controller.Enqueued)
	if !assert.NoError(t, check.Reconcile()) {
		return
	}
	health, err := healthset.HealthV1beta1().ComponentHealths().Get(ctx, volumeRemoteBackupComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, 2, health.Status.Checks[checkKeyVolumeRemoteBackupFailed].AffectedCount)

	obj.Status.Conditions = []harvesterv1.Condition{{Type: volumeRemoteReady, Status: corev1.ConditionTrue}}
	if _, err := clientset.HarvesterhciV1beta1().VolumeRemoteBackups(obj.Namespace).Update(ctx, obj, metav1.UpdateOptions{}); !assert.NoError(t, err) {
		return
	}
	if !assert.NoError(t, check.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, volumeRemoteBackupComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	got := health.Status.Checks[checkKeyVolumeRemoteBackupFailed]
	assert.Equal(t, 1, got.AffectedCount)
	assert.NotContains(t, got.AffectedResources.Names, "default/obj-a")

	if !assert.NoError(t, clientset.HarvesterhciV1beta1().VolumeRemoteBackups(other.Namespace).Delete(ctx, other.Name, metav1.DeleteOptions{})) {
		return
	}
	result, err = check.OnVolumeRemoteBackupChanged("default/obj-b", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
	if !assert.NoError(t, check.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, volumeRemoteBackupComponentHealthName, metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, health.Status.Checks)
	}
}

func TestReconcileVolumeRemoteBackupsListError(t *testing.T) {
	clientset, _ := testutil.NewClients()
	clientset.PrependReactor("list", "volumeremotebackups", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("cache failure")
	})
	check := &volumeRemoteBackupCheck{cache: fakeclients.VolumeRemoteBackupCache(clientset.HarvesterhciV1beta1().VolumeRemoteBackups)}
	assert.EqualError(t, check.Reconcile(), "failed to list VolumeRemoteBackups: cache failure")
}
