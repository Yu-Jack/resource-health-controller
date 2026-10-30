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

func TestReconcileUpgradeLogs(t *testing.T) {
	tests := []struct {
		name       string
		conditions []harvesterv1.Condition
		keys       []string
		deleting   bool
	}{
		{name: "new upgrade log"},
		{name: "deleting upgrade log is excluded", deleting: true, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.InfraReady, Status: corev1.ConditionFalse, Message: "fluentd failed"},
		}},
		{name: "infrastructure in progress", conditions: []harvesterv1.Condition{
			{Type: harvesterv1.LoggingOperatorDeployed, Status: corev1.ConditionTrue},
			{Type: harvesterv1.InfraReady, Status: corev1.ConditionUnknown},
		}},
		{name: "ready with no archive to download", conditions: []harvesterv1.Condition{
			{Type: harvesterv1.LoggingOperatorDeployed, Status: corev1.ConditionTrue},
			{Type: harvesterv1.InfraReady, Status: corev1.ConditionTrue},
			{Type: harvesterv1.UpgradeLogReady, Status: corev1.ConditionTrue},
			{Type: harvesterv1.DownloadReady, Status: corev1.ConditionFalse},
		}},
		{name: "operator not ready", keys: []string{checkKeyUpgradeLogOperatorNotReady}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.LoggingOperatorDeployed, Status: corev1.ConditionFalse, Message: "fluentd failed"},
		}},
		{name: "infra not ready", keys: []string{checkKeyUpgradeLogInfraNotReady}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.InfraReady, Status: corev1.ConditionFalse, Message: "fluentd failed"},
		}},
		{name: "not started", keys: []string{checkKeyUpgradeLogNotStarted}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.UpgradeLogReady, Status: corev1.ConditionFalse, Message: "fluentd failed"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			upgradeLog := &harvesterv1.UpgradeLog{
				ObjectMeta: metav1.ObjectMeta{Name: "upgradelog-a", Namespace: "harvester-system"},
				Spec:       harvesterv1.UpgradeLogSpec{UpgradeName: "hvst-upgrade-x8k2p"},
				Status:     harvesterv1.UpgradeLogStatus{Conditions: tc.conditions},
			}
			if tc.deleting {
				deletedAt := metav1.Now()
				upgradeLog.DeletionTimestamp = &deletedAt
			}
			clientset, healthset := testutil.NewClients(upgradeLog)
			writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			upgradeLogs := &upgradeLogCheck{writer: writer, upgradeLogCache: fakeclients.UpgradeLogCache(clientset.HarvesterhciV1beta1().UpgradeLogs)}
			if !assert.NoError(t, upgradeLogs.Reconcile()) {
				return
			}
			health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), upgradeLogComponentHealthName, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			assert.Len(t, health.Status.Checks, len(tc.keys))
			for _, key := range tc.keys {
				check := health.Status.Checks[key]
				assert.Equal(t, healthv1.SeverityWarning, check.Severity, key)
				assert.Equal(t, 1, check.AffectedCount, key)
				assert.Equal(t, upgradeLogKind, check.AffectedResources.Kind)
				assert.Equal(t, harvesterv1.SchemeGroupVersion.String(), check.AffectedResources.APIVersion)
				assert.Contains(t, check.AffectedResources.Names, "harvester-system/upgradelog-a")
				assert.Contains(t, check.Message, `message="fluentd failed", upgrade="hvst-upgrade-x8k2p"`)
			}
		})
	}
}

func TestUpgradeLogChecksClearAndPreserveNamespacedResources(t *testing.T) {
	ctx := context.Background()
	upgradeLog := &harvesterv1.UpgradeLog{
		ObjectMeta: metav1.ObjectMeta{Name: "upgradelog-a", Namespace: "harvester-system"},
		Status: harvesterv1.UpgradeLogStatus{Conditions: []harvesterv1.Condition{
			{Type: harvesterv1.InfraReady, Status: corev1.ConditionFalse, Message: "fluentd failed"},
		}},
	}
	other := upgradeLog.DeepCopy()
	other.Name = "upgradelog-b"
	clientset, healthset := testutil.NewClients(upgradeLog, other)
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller: controller,
		Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
	}
	upgradeLogs := &upgradeLogCheck{writer: writer, upgradeLogCache: fakeclients.UpgradeLogCache(clientset.HarvesterhciV1beta1().UpgradeLogs)}
	result, err := upgradeLogs.OnUpgradeLogChanged("harvester-system/upgradelog-a", upgradeLog)
	assert.Same(t, upgradeLog, result)
	assert.NoError(t, err)
	assert.Equal(t, []string{upgradeLogComponentHealthName}, controller.Enqueued)
	if !assert.NoError(t, upgradeLogs.Reconcile()) {
		return
	}
	health, err := healthset.HealthV1beta1().ComponentHealths().Get(ctx, upgradeLogComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, 2, health.Status.Checks[checkKeyUpgradeLogInfraNotReady].AffectedCount)

	upgradeLog.Status.Conditions = []harvesterv1.Condition{{Type: harvesterv1.InfraReady, Status: corev1.ConditionTrue}}
	if _, err := clientset.HarvesterhciV1beta1().UpgradeLogs(upgradeLog.Namespace).Update(ctx, upgradeLog, metav1.UpdateOptions{}); !assert.NoError(t, err) {
		return
	}
	if !assert.NoError(t, upgradeLogs.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, upgradeLogComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	check := health.Status.Checks[checkKeyUpgradeLogInfraNotReady]
	assert.Equal(t, 1, check.AffectedCount)
	assert.NotContains(t, check.AffectedResources.Names, "harvester-system/upgradelog-a")

	if !assert.NoError(t, clientset.HarvesterhciV1beta1().UpgradeLogs(other.Namespace).Delete(ctx, other.Name, metav1.DeleteOptions{})) {
		return
	}
	result, err = upgradeLogs.OnUpgradeLogChanged("harvester-system/upgradelog-b", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
	if !assert.NoError(t, upgradeLogs.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, upgradeLogComponentHealthName, metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, health.Status.Checks)
	}
}

func TestReconcileUpgradeLogsListError(t *testing.T) {
	clientset, _ := testutil.NewClients()
	clientset.PrependReactor("list", "upgradelogs", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("cache failure")
	})
	upgradeLogs := &upgradeLogCheck{upgradeLogCache: fakeclients.UpgradeLogCache(clientset.HarvesterhciV1beta1().UpgradeLogs)}
	assert.EqualError(t, upgradeLogs.Reconcile(), "failed to list UpgradeLogs: cache failure")
}
