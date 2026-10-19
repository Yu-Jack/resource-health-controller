package componenthealth

import (
	"context"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func newScheduleVMBackup(name string, conditions []harvesterv1.Condition) *harvesterv1.ScheduleVMBackup {
	return &harvesterv1.ScheduleVMBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: harvesterv1.ScheduleVMBackupStatus{
			Conditions: conditions,
		},
	}
}

func TestReconcileScheduleVMBackups(t *testing.T) {
	tests := []struct {
		name             string
		scheduleVMBackup *harvesterv1.ScheduleVMBackup
		expectedChecks   int
		assertCheck      func(*testing.T, healthv1.CheckResult)
	}{
		{
			name: "true condition reason creates warning check",
			scheduleVMBackup: newScheduleVMBackup("suspended-svmbackup", []harvesterv1.Condition{
				{
					Type:    condition.Cond("BackupSuspend"),
					Status:  corev1.ConditionTrue,
					Reason:  "Reach Max Failure",
					Message: "failure backups 2 reach max tolerance 2",
				},
			}),
			expectedChecks: 1,
			assertCheck: func(t *testing.T, check healthv1.CheckResult) {
				assert.Equal(t, healthv1.SeverityError, check.Severity)
				assert.Equal(t, "1 ScheduleVMBackup(s) have Reach Max Failure: failure backups 2 reach max tolerance 2", check.Message)
				assert.Equal(t, 1, check.AffectedCount)
				assert.Equal(t, harvesterv1.SchemeGroupVersion.String(), check.AffectedResources.APIVersion)
				assert.Equal(t, scheduleVMBackupKind, check.AffectedResources.Kind)
				assert.Contains(t, check.AffectedResources.Names, "suspended-svmbackup")
			},
		},
		{
			name: "false condition reason is ignored",
			scheduleVMBackup: newScheduleVMBackup("healthy-svmbackup", []harvesterv1.Condition{
				{
					Type:   condition.Cond("BackupSuspend"),
					Status: corev1.ConditionFalse,
					Reason: "Reach Max Failure",
				},
			}),
			expectedChecks: 0,
		},
		{
			name:             "no conditions creates no check",
			scheduleVMBackup: newScheduleVMBackup("no-condition-svmbackup", nil),
			expectedChecks:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset, healthset := newTestClients(tc.scheduleVMBackup)
			handler := &Handler{
				componentHealths:      fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
				scheduleVMBackupCache: fakeclients.SVMBackupCache(clientset.HarvesterhciV1beta1().ScheduleVMBackups),
			}

			err := handler.reconcileScheduleVMBackups()
			assert.NoError(t, err)

			componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), scheduleVMBackupComponentHealthName, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Len(t, componentHealth.Status.Checks, tc.expectedChecks)
			if tc.assertCheck != nil {
				check, ok := componentHealth.Status.Checks[tc.scheduleVMBackup.Status.Conditions[0].Reason]
				assert.True(t, ok)
				tc.assertCheck(t, check)
			}
		})
	}
}
