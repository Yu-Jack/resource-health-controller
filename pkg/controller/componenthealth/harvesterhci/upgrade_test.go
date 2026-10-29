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
	"sigs.k8s.io/yaml"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

// upgradingNodesYAML is an Upgrade of a cluster that is upgrading its nodes, as it is stored in the cluster.
const upgradingNodesYAML = `
apiVersion: harvesterhci.io/v1beta1
kind: Upgrade
metadata:
  annotations:
    harvesterhci.io/apply-skip-rke2-manifests-plan-completed: "true"
  creationTimestamp: "2026-10-20T03:10:12Z"
  finalizers:
  - wrangler.cattle.io/harvester-upgrade-controller
  generateName: hvst-upgrade-
  generation: 1
  labels:
    harvesterhci.io/latestUpgrade: "true"
    harvesterhci.io/upgradeState: UpgradingNodes
  name: hvst-upgrade-x8k2p
  namespace: harvester-system
spec:
  logEnabled: true
  version: v1.8.0
status:
  conditions:
  - status: Unknown
    type: Completed
  - lastUpdateTime: "2026-10-20T03:10:44Z"
    status: "True"
    type: LogReady
  - lastUpdateTime: "2026-10-20T03:12:15Z"
    status: "True"
    type: ImageReady
  - lastUpdateTime: "2026-10-20T03:14:56Z"
    status: "True"
    type: RepoReady
  - lastUpdateTime: "2026-10-20T03:28:09Z"
    status: "True"
    type: NodesPrepared
  - lastUpdateTime: "2026-10-20T03:40:17Z"
    status: "True"
    type: SystemServicesUpgraded
  - status: Unknown
    type: NodesUpgraded
  imageID: harvester-system/hvst-upgrade-x8k2p
  nodeStatuses:
    harvester-node-01:
      state: Succeeded
    harvester-node-02:
      state: Succeeded
    harvester-node-03:
      state: Pre-draining
  previousVersion: v1.7.2
  upgradeLog: hvst-upgrade-x8k2p-upgradelog
`

func TestReconcileUpgradingNodesUpgrade(t *testing.T) {
	upgrade := &harvesterv1.Upgrade{}
	if !assert.NoError(t, yaml.Unmarshal([]byte(upgradingNodesYAML), upgrade)) {
		return
	}
	reconcile := func(upgrade *harvesterv1.Upgrade) *healthv1.ComponentHealth {
		clientset, healthset := testutil.NewClients(upgrade)
		writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
		upgrades := &upgradeCheck{writer: writer, upgradeCache: fakeclients.UpgradeCache(clientset.HarvesterhciV1beta1().Upgrades)}
		if !assert.NoError(t, upgrades.Reconcile()) {
			return &healthv1.ComponentHealth{}
		}
		health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), upgradeComponentHealthName, metav1.GetOptions{})
		if !assert.NoError(t, err) {
			return &healthv1.ComponentHealth{}
		}
		return health
	}

	t.Run("upgrading nodes is not a failure", func(t *testing.T) {
		assert.Empty(t, reconcile(upgrade).Status.Checks)
	})

	t.Run("a node fails to upgrade", func(t *testing.T) {
		failed := upgrade.DeepCopy()
		failed.Labels["harvesterhci.io/upgradeState"] = "Failed"
		failed.Status.NodeStatuses["harvester-node-03"] = harvesterv1.NodeUpgradeStatus{
			State: "Failed", Reason: "JobFailed", Message: "job has reached the specified backoff limit",
		}
		failed.Status.Conditions = []harvesterv1.Condition{
			{Type: harvesterv1.UpgradeCompleted, Status: corev1.ConditionFalse},
			{Type: harvesterv1.LogReady, Status: corev1.ConditionTrue},
			{Type: harvesterv1.ImageReady, Status: corev1.ConditionTrue},
			{Type: harvesterv1.RepoProvisioned, Status: corev1.ConditionTrue},
			{Type: harvesterv1.NodesPrepared, Status: corev1.ConditionTrue},
			{Type: harvesterv1.SystemServicesUpgraded, Status: corev1.ConditionTrue},
			{Type: harvesterv1.NodesUpgraded, Status: corev1.ConditionFalse, Reason: "JobFailed", Message: "job has reached the specified backoff limit"},
		}

		health := reconcile(failed)
		out, _ := yaml.Marshal(health)
		fmt.Println(string(out))
		checks := health.Status.Checks
		assert.Len(t, checks, 2)
		assert.Contains(t, checks, checkKeyUpgradeFailed)
		check := checks[checkKeyUpgradeNodesUpgradeFailed]
		assert.Equal(t, healthv1.SeverityError, check.Severity)
		assert.Len(t, check.AffectedResources.Names, 1)
		assert.Contains(t, check.AffectedResources.Names, "harvester-system/hvst-upgrade-x8k2p")
		assert.Contains(t, check.Message, `previousVersion="v1.7.2", version="v1.8.0"`)
		assert.Contains(t, check.Message, `node harvester-node-03: reason="JobFailed", message="job has reached the specified backoff limit"`)
		assert.NotContains(t, check.Message, "harvester-node-01")
	})
}

func TestReconcileUpgrades(t *testing.T) {
	tests := []struct {
		name       string
		conditions []harvesterv1.Condition
		keys       map[string]healthv1.Severity
		deleting   bool
	}{
		{name: "new upgrade"},
		{name: "deleting upgrade is excluded", deleting: true, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.UpgradeCompleted, Status: corev1.ConditionFalse, Message: "failed"},
		}},
		{name: "upgrade in progress", conditions: []harvesterv1.Condition{
			{Type: harvesterv1.UpgradeCompleted, Status: corev1.ConditionUnknown},
			{Type: harvesterv1.ImageReady, Status: corev1.ConditionTrue},
			{Type: harvesterv1.NodesUpgraded, Status: corev1.ConditionUnknown},
		}},
		{name: "upgrade succeeded", conditions: []harvesterv1.Condition{
			{Type: harvesterv1.UpgradeCompleted, Status: corev1.ConditionTrue},
			{Type: harvesterv1.NodesUpgraded, Status: corev1.ConditionTrue},
		}},
		{name: "image not ready", keys: map[string]healthv1.Severity{
			checkKeyUpgradeFailed:        healthv1.SeverityError,
			checkKeyUpgradeImageNotReady: healthv1.SeverityError,
		}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.UpgradeCompleted, Status: corev1.ConditionFalse},
			{Type: harvesterv1.ImageReady, Status: corev1.ConditionFalse, Message: "download failed"},
		}},
		{name: "repo not provisioned", keys: map[string]healthv1.Severity{checkKeyUpgradeRepoNotProvisioned: healthv1.SeverityError}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.RepoProvisioned, Status: corev1.ConditionFalse, Message: "repo failed"},
		}},
		{name: "nodes prepare failed", keys: map[string]healthv1.Severity{checkKeyUpgradeNodesPrepareFailed: healthv1.SeverityError}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.NodesPrepared, Status: corev1.ConditionFalse, Message: "prepare failed"},
		}},
		{name: "nodes upgrade failed", keys: map[string]healthv1.Severity{checkKeyUpgradeNodesUpgradeFailed: healthv1.SeverityError}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.NodesUpgraded, Status: corev1.ConditionFalse, Message: "node failed"},
		}},
		{name: "system services failed", keys: map[string]healthv1.Severity{checkKeyUpgradeSystemServicesFailed: healthv1.SeverityError}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.SystemServicesUpgraded, Status: corev1.ConditionFalse, Message: "chart failed"},
		}},
		{name: "log not ready is a warning", keys: map[string]healthv1.Severity{checkKeyUpgradeLogNotReady: healthv1.SeverityWarning}, conditions: []harvesterv1.Condition{
			{Type: harvesterv1.LogReady, Status: corev1.ConditionFalse, Message: "log failed"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			upgrade := &harvesterv1.Upgrade{
				ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "harvester-system"},
				Spec:       harvesterv1.UpgradeSpec{Version: "v1.9.0"},
				Status: harvesterv1.UpgradeStatus{
					Conditions:      tc.conditions,
					PreviousVersion: "v1.8.0",
					NodeStatuses: map[string]harvesterv1.NodeUpgradeStatus{
						"node-b": {State: "Succeeded"},
						"node-a": {State: "Failed", Reason: "JobFailed", Message: "job exceeded backoff"},
					},
				},
			}
			if tc.deleting {
				deletedAt := metav1.Now()
				upgrade.DeletionTimestamp = &deletedAt
			}
			clientset, healthset := testutil.NewClients(upgrade)
			writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			upgrades := &upgradeCheck{writer: writer, upgradeCache: fakeclients.UpgradeCache(clientset.HarvesterhciV1beta1().Upgrades)}
			if !assert.NoError(t, upgrades.Reconcile()) {
				return
			}
			health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), upgradeComponentHealthName, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			assert.Len(t, health.Status.Checks, len(tc.keys))
			for key, severity := range tc.keys {
				check := health.Status.Checks[key]
				assert.Equal(t, severity, check.Severity, key)
				assert.Equal(t, 1, check.AffectedCount, key)
				assert.Equal(t, upgradeKind, check.AffectedResources.Kind)
				assert.Equal(t, harvesterv1.SchemeGroupVersion.String(), check.AffectedResources.APIVersion)
				assert.Contains(t, check.AffectedResources.Names, "harvester-system/upgrade")
				assert.Contains(t, check.Message, `previousVersion="v1.8.0", version="v1.9.0"`)
			}
			if _, ok := tc.keys[checkKeyUpgradeNodesUpgradeFailed]; ok {
				message := health.Status.Checks[checkKeyUpgradeNodesUpgradeFailed].Message
				assert.Contains(t, message, `node node-a: reason="JobFailed", message="job exceeded backoff"`)
				assert.NotContains(t, message, "node-b")
			}
		})
	}
}

func TestUpgradeChecksClearAndPreserveNamespacedResources(t *testing.T) {
	ctx := context.Background()
	upgrade := &harvesterv1.Upgrade{
		ObjectMeta: metav1.ObjectMeta{Name: "upgrade", Namespace: "harvester-system"},
		Status: harvesterv1.UpgradeStatus{Conditions: []harvesterv1.Condition{
			{Type: harvesterv1.ImageReady, Status: corev1.ConditionFalse, Message: "download failed"},
		}},
	}
	other := upgrade.DeepCopy()
	other.Name = "other"
	clientset, healthset := testutil.NewClients(upgrade, other)
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller: controller,
		Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
	}
	upgrades := &upgradeCheck{writer: writer, upgradeCache: fakeclients.UpgradeCache(clientset.HarvesterhciV1beta1().Upgrades)}
	result, err := upgrades.OnUpgradeChanged("harvester-system/upgrade", upgrade)
	assert.Same(t, upgrade, result)
	assert.NoError(t, err)
	assert.Equal(t, []string{upgradeComponentHealthName}, controller.Enqueued)
	if !assert.NoError(t, upgrades.Reconcile()) {
		return
	}
	health, err := healthset.HealthV1beta1().ComponentHealths().Get(ctx, upgradeComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	check := health.Status.Checks[checkKeyUpgradeImageNotReady]
	assert.Equal(t, 2, check.AffectedCount)
	assert.Contains(t, check.AffectedResources.Names, "harvester-system/upgrade")
	assert.Contains(t, check.AffectedResources.Names, "harvester-system/other")

	upgrade.Status.Conditions = []harvesterv1.Condition{{Type: harvesterv1.ImageReady, Status: corev1.ConditionTrue}}
	if _, err := clientset.HarvesterhciV1beta1().Upgrades(upgrade.Namespace).Update(ctx, upgrade, metav1.UpdateOptions{}); !assert.NoError(t, err) {
		return
	}
	if !assert.NoError(t, upgrades.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, upgradeComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	check = health.Status.Checks[checkKeyUpgradeImageNotReady]
	assert.Equal(t, 1, check.AffectedCount)
	assert.NotContains(t, check.AffectedResources.Names, "harvester-system/upgrade")

	if !assert.NoError(t, clientset.HarvesterhciV1beta1().Upgrades(other.Namespace).Delete(ctx, other.Name, metav1.DeleteOptions{})) {
		return
	}
	result, err = upgrades.OnUpgradeChanged("harvester-system/other", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
	if !assert.NoError(t, upgrades.Reconcile()) {
		return
	}
	health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, upgradeComponentHealthName, metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, health.Status.Checks)
	}
}

func TestReconcileUpgradesListError(t *testing.T) {
	clientset, _ := testutil.NewClients()
	clientset.PrependReactor("list", "upgrades", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("cache failure")
	})
	upgrades := &upgradeCheck{upgradeCache: fakeclients.UpgradeCache(clientset.HarvesterhciV1beta1().Upgrades)}
	assert.EqualError(t, upgrades.Reconcile(), "failed to list Upgrades: cache failure")
}
