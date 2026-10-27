package componenthealth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/util/fakeclients"
)

func newVMBackup(name string, volumeBackups []harvesterv1.VolumeBackup) *harvesterv1.VirtualMachineBackup {
	return &harvesterv1.VirtualMachineBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Status: harvesterv1.VirtualMachineBackupStatus{
			VolumeBackups: volumeBackups,
		},
	}
}

func errorMessage(message string) *string {
	return &message
}

func TestOnVMBackupChangedAfterDeletion(t *testing.T) {
	tests := []struct {
		name             string
		remainingFailure bool
	}{
		{name: "deleting the last failed backup clears the check"},
		{name: "deleting one failed backup preserves remaining failures", remainingFailure: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			backup := newVMBackup("deleted-backup", []harvesterv1.VolumeBackup{
				{Error: &harvesterv1.Error{Message: errorMessage("backup failed")}},
			})
			clientset, healthset := newTestClients(backup)
			if tc.remainingFailure {
				remaining := backup.DeepCopy()
				remaining.Name = "remaining-backup"
				_, err := clientset.HarvesterhciV1beta1().VirtualMachineBackups("default").Create(ctx, remaining, metav1.CreateOptions{})
				if !assert.NoError(t, err) {
					return
				}
			}
			controller := &recordingComponentHealthController{}
			handler := &Handler{}
			writer := componentHealthWriter{
				controller: controller,
				client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
			}
			backups := &vmBackupCheck{writer: writer, vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)}
			handler.checks = []healthCheck{backups}
			_, err := backups.OnVMBackupChanged("default/deleted-backup", backup)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Equal(t, []string{vmBackupComponentHealthName}, controller.enqueued) {
				return
			}
			_, err = handler.OnComponentHealthChanged(controller.enqueued[0], nil)
			if !assert.NoError(t, err) {
				return
			}
			health, err := healthset.HealthV1beta1().ComponentHealths().Get(ctx, vmBackupComponentHealthName, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			assert.Contains(t, health.Status.Checks, checkKeyVMBackupFailed)

			err = clientset.HarvesterhciV1beta1().VirtualMachineBackups("default").Delete(ctx, backup.Name, metav1.DeleteOptions{})
			if !assert.NoError(t, err) {
				return
			}
			result, err := backups.OnVMBackupChanged("default/deleted-backup", nil)
			assert.Nil(t, result)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Equal(t, []string{vmBackupComponentHealthName, vmBackupComponentHealthName}, controller.enqueued) {
				return
			}
			_, err = handler.OnComponentHealthChanged(controller.enqueued[1], health)
			if !assert.NoError(t, err) {
				return
			}
			health, err = healthset.HealthV1beta1().ComponentHealths().Get(ctx, vmBackupComponentHealthName, metav1.GetOptions{})
			if !assert.NoError(t, err) {
				return
			}
			if tc.remainingFailure {
				check := health.Status.Checks[checkKeyVMBackupFailed]
				assert.Equal(t, 1, check.AffectedCount)
				assert.Contains(t, check.AffectedResources.Names, "remaining-backup")
				assert.NotContains(t, check.AffectedResources.Names, backup.Name)
			} else {
				assert.Empty(t, health.Status.Checks)
			}
		})
	}
}

func TestReconcileVMBackups(t *testing.T) {
	older := metav1.NewTime(metav1.Now().Add(-1 * 60 * 1e9)) // 1 minute earlier
	newer := metav1.Now()

	tests := []struct {
		name           string
		vmBackup       *harvesterv1.VirtualMachineBackup
		expectedChecks int
		assertCheck    func(*testing.T, healthv1.CheckResult)
	}{
		{
			name: "latest volume backup with error creates check",
			vmBackup: newVMBackup("failed-vmbackup", []harvesterv1.VolumeBackup{
				{
					CreationTime: &older,
					Error:        &harvesterv1.Error{Message: errorMessage("old error")},
				},
				{
					CreationTime: &newer,
					Error:        &harvesterv1.Error{Message: errorMessage("latest error")},
				},
			}),
			expectedChecks: 1,
			assertCheck: func(t *testing.T, check healthv1.CheckResult) {
				assert.Equal(t, healthv1.SeverityError, check.Severity)
				assert.Equal(t, "1 VirtualMachineBackup(s) failed: latest error", check.Message)
				assert.Equal(t, 1, check.AffectedCount)
				assert.Equal(t, harvesterv1.SchemeGroupVersion.String(), check.AffectedResources.APIVersion)
				assert.Equal(t, vmBackupKind, check.AffectedResources.Kind)
				assert.Contains(t, check.AffectedResources.Names, "failed-vmbackup")
			},
		},
		{
			name:           "no volume backups creates no check",
			vmBackup:       newVMBackup("healthy-vmbackup", nil),
			expectedChecks: 0,
		},
		{
			name: "volume backup without error creates no check",
			vmBackup: newVMBackup("healthy-vmbackup", []harvesterv1.VolumeBackup{
				{CreationTime: &newer},
			}),
			expectedChecks: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset, healthset := newTestClients(tc.vmBackup)
			writer := componentHealthWriter{client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			backups := &vmBackupCheck{writer: writer, vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)}

			err := backups.reconcile()
			assert.NoError(t, err)

			componentHealth, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), vmBackupComponentHealthName, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Len(t, componentHealth.Status.Checks, tc.expectedChecks)
			if tc.assertCheck != nil {
				check, ok := componentHealth.Status.Checks[checkKeyVMBackupFailed]
				assert.True(t, ok)
				tc.assertCheck(t, check)
			}
		})
	}
}
