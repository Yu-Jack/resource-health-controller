package longhorn

import (
	"context"
	"testing"
	"time"

	longhornv1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

const testCheckKey = "ReplicaSchedulingFailure"

func TestOnVolumeChangedAfterDeletionClearsComponentHealthChecks(t *testing.T) {
	clientset, healthset := testutil.NewClients(&healthv1.ComponentHealth{
		ObjectMeta: metav1.ObjectMeta{Name: volumeComponentHealthName},
		Status: healthv1.ComponentHealthStatus{
			Checks: map[string]healthv1.CheckResult{
				"StaleCheck": {Severity: healthv1.SeverityError, Message: "deleted resource failed"},
			},
		},
	})
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller: controller,
		Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
	}
	volumes := &volumeCheck{writer: writer, volumeCache: fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes)}

	result, err := volumes.OnVolumeChanged("longhorn-system/deleted-volume", nil)
	assert.Nil(t, result)
	assert.NoError(t, err)
	if !assert.Equal(t, []string{volumeComponentHealthName}, controller.Enqueued) {
		return
	}

	assert.NoError(t, volumes.Reconcile())

	health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	if assert.NoError(t, err) {
		assert.Empty(t, health.Status.Checks)
	}
}

func TestReconcileVolumes(t *testing.T) {
	tests := []struct {
		name           string
		volume         *longhornv1.Volume
		expectedChecks int
		assertCheck    func(*testing.T, healthv1.CheckResult)
	}{
		{
			name: "false condition reason creates warning check",
			volume: newLonghornVolume("pvc-replica-scheduling-failure", []longhornv1.Condition{
				{
					Type:    longhornv1.VolumeConditionTypeScheduled,
					Status:  longhornv1.ConditionStatusFalse,
					Reason:  longhornv1.VolumeConditionReasonReplicaSchedulingFailure,
					Message: "precheck new replica failed: disks are unavailable",
				},
			}),
			expectedChecks: 1,
			assertCheck: func(t *testing.T, check healthv1.CheckResult) {
				assert.Equal(t, healthv1.SeverityWarning, check.Severity)
				assert.Equal(t, "1 Longhorn Volume(s) have ReplicaSchedulingFailure: precheck new replica failed: disks are unavailable", check.Message)
				assert.Equal(t, 1, check.AffectedCount)
				assert.Equal(t, longhornv1.SchemeGroupVersion.String(), check.AffectedResources.APIVersion)
				assert.Equal(t, volumeKind, check.AffectedResources.Kind)
				assert.Contains(t, check.AffectedResources.Names, "pvc-replica-scheduling-failure")
			},
		},
		{
			name: "another false condition reason creates check with reason key",
			volume: newLonghornVolume("pvc-local-replica-scheduling-failure", []longhornv1.Condition{
				{
					Type:   longhornv1.VolumeConditionTypeScheduled,
					Status: longhornv1.ConditionStatusFalse,
					Reason: longhornv1.VolumeConditionReasonLocalReplicaSchedulingFailure,
				},
			}),
			expectedChecks: 1,
			assertCheck: func(t *testing.T, check healthv1.CheckResult) {
				assert.Equal(t, "1 Longhorn Volume(s) have LocalReplicaSchedulingFailure", check.Message)
				assert.Contains(t, check.AffectedResources.Names, "pvc-local-replica-scheduling-failure")
			},
		},
		{
			name: "non scheduled false condition reason creates check",
			volume: newLonghornVolume("pvc-restore-failure", []longhornv1.Condition{
				{
					Type:   longhornv1.VolumeConditionTypeRestore,
					Status: longhornv1.ConditionStatusFalse,
					Reason: longhornv1.VolumeConditionReasonRestoreFailure,
				},
			}),
			expectedChecks: 1,
			assertCheck: func(t *testing.T, check healthv1.CheckResult) {
				assert.Equal(t, "1 Longhorn Volume(s) have RestoreFailure", check.Message)
				assert.Contains(t, check.AffectedResources.Names, "pvc-restore-failure")
			},
		},
		{
			name: "true condition reason is ignored",
			volume: newLonghornVolume("healthy-volume", []longhornv1.Condition{
				{
					Type:   longhornv1.VolumeConditionTypeScheduled,
					Status: longhornv1.ConditionStatusTrue,
					Reason: longhornv1.VolumeConditionReasonReplicaSchedulingFailure,
				},
			}),
			expectedChecks: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset, healthset := testutil.NewClients(tc.volume)
			writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			volumes := &volumeCheck{writer: writer, volumeCache: fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes)}

			err := volumes.Reconcile()
			assert.NoError(t, err)

			componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, healthcheck.ComponentName, componentHealth.Labels[healthv1.LabelKeyComponent])
			assert.Len(t, componentHealth.Status.Checks, tc.expectedChecks)
			if tc.assertCheck != nil {
				check, ok := componentHealth.Status.Checks[tc.volume.Status.Conditions[0].Reason]
				assert.True(t, ok)
				tc.assertCheck(t, check)
			}
		})
	}
}

func TestReconcileVolumesReplacesExistingChecks(t *testing.T) {
	clientset, healthset := testutil.NewClients(
		&healthv1.ComponentHealth{
			ObjectMeta: metav1.ObjectMeta{Name: volumeComponentHealthName},
			Status: healthv1.ComponentHealthStatus{
				Checks: map[string]healthv1.CheckResult{
					"OtherVolumeCheck": {
						Severity: healthv1.SeverityWarning,
						Message:  "existing check of another reconcile function",
					},
					longhornv1.VolumeConditionReasonReplicaSchedulingFailure: {
						Severity: healthv1.SeverityWarning,
						Message:  "stale volume check",
						AffectedResources: &healthv1.AffectedResources{
							APIVersion: longhornv1.SchemeGroupVersion.String(),
							Kind:       volumeKind,
						},
					},
				},
			},
		},
		newLonghornVolume("healthy-volume", []longhornv1.Condition{
			{
				Type:   longhornv1.VolumeConditionTypeScheduled,
				Status: longhornv1.ConditionStatusTrue,
			},
		}),
	)
	writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
	volumes := &volumeCheck{writer: writer, volumeCache: fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes)}

	err := volumes.Reconcile()
	assert.NoError(t, err)

	componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, healthcheck.ComponentName, componentHealth.Labels[healthv1.LabelKeyComponent])
	assert.NotContains(t, componentHealth.Status.Checks, "OtherVolumeCheck")
	assert.NotContains(t, componentHealth.Status.Checks, longhornv1.VolumeConditionReasonReplicaSchedulingFailure)
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
	clientset, healthset := testutil.NewClients(
		volumeWithFalseCondition("pvc-aaa"),
		volumeWithFalseCondition("pvc-bbb"),
		testutil.NewIgnoreRule(volumeComponentHealthName, "ignore-aaa", testCheckKey, []string{"pvc-aaa"}, &expiresAt),
		// targets another ComponentHealth
		&healthv1.HealthIgnoreRule{
			ObjectMeta: metav1.ObjectMeta{Name: "other"},
			Spec:       healthv1.HealthIgnoreRuleSpec{ComponentHealth: "harvester-controller-node", CheckKey: testCheckKey},
		},
	)
	controller := &testutil.RecordingController{}
	writer := healthcheck.Writer{
		Controller:  controller,
		Client:      fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
		IgnoreRules: fakeclients.HealthIgnoreRuleCache(healthset.HealthV1beta1().HealthIgnoreRules),
	}
	volumes := &volumeCheck{writer: writer, volumeCache: fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes)}

	assert.NoError(t, volumes.Reconcile())

	componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	check := componentHealth.Status.Checks[testCheckKey]
	assert.False(t, check.Ignored)
	assert.Equal(t, 2, check.AffectedCount)
	assert.Equal(t, healthv1.AffectedResourceDetail{Ignored: true, IgnoredBy: "ignore-aaa"}, check.AffectedResources.Names["pvc-aaa"])
	assert.Equal(t, healthv1.AffectedResourceDetail{}, check.AffectedResources.Names["pvc-bbb"])
	// a reconcile is scheduled for the expiry
	assert.Equal(t, []string{volumeComponentHealthName}, controller.EnqueuedAfter)

	// the rule is deleted: the mark goes away
	assert.NoError(t, healthset.HealthV1beta1().HealthIgnoreRules().Delete(context.Background(), "ignore-aaa", metav1.DeleteOptions{}))
	assert.NoError(t, volumes.Reconcile())
	componentHealth, err = healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, healthv1.AffectedResourceDetail{}, componentHealth.Status.Checks[testCheckKey].AffectedResources.Names["pvc-aaa"])
}

func TestReconcileVolumesIgnoreIsStable(t *testing.T) {
	clientset, healthset := testutil.NewClients(
		volumeWithFalseCondition("pvc-aaa"),
		testutil.NewIgnoreRule(volumeComponentHealthName, "ignore-all", testCheckKey, nil, nil),
	)
	writer := healthcheck.Writer{
		Client:      fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
		IgnoreRules: fakeclients.HealthIgnoreRuleCache(healthset.HealthV1beta1().HealthIgnoreRules),
	}
	volumes := &volumeCheck{writer: writer, volumeCache: fakeclients.LonghornVolumeCache(clientset.LonghornV1beta2().Volumes)}

	assert.NoError(t, volumes.Reconcile())
	first, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), volumeComponentHealthName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.True(t, first.Status.Checks[testCheckKey].Ignored)

	// nothing changed: the second reconcile must not write again
	healthset.ClearActions()
	assert.NoError(t, volumes.Reconcile())
	for _, action := range healthset.Actions() {
		assert.NotEqual(t, "update", action.GetVerb())
	}
}

func newLonghornVolume(name string, conditions []longhornv1.Condition) *longhornv1.Volume {
	return &longhornv1.Volume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: longhornNamespace,
		},
		Status: longhornv1.VolumeStatus{
			Conditions: conditions,
		},
	}
}
