package harvesterhci

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/testutil"
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

func TestOnVMBackupChangedEnqueuesComponentHealth(t *testing.T) {
	controller := &testutil.RecordingController{}
	backups := &vmBackupCheck{writer: healthcheck.Writer{Controller: controller}}
	for index := 0; index < 100; index++ {
		backup := newVMBackup(fmt.Sprintf("backup-%d", index), nil)
		result, err := backups.OnVMBackupChanged("default/"+backup.Name, backup)
		assert.Same(t, backup, result)
		assert.NoError(t, err)
	}
	assert.Len(t, controller.Enqueued, 100)
	for _, name := range controller.Enqueued {
		assert.Equal(t, vmBackupComponentHealthName, name)
	}
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
			clientset, healthset := testutil.NewClients(backup)
			if tc.remainingFailure {
				remaining := backup.DeepCopy()
				remaining.Name = "remaining-backup"
				_, err := clientset.HarvesterhciV1beta1().VirtualMachineBackups("default").Create(ctx, remaining, metav1.CreateOptions{})
				if !assert.NoError(t, err) {
					return
				}
			}
			controller := &testutil.RecordingController{}
			writer := healthcheck.Writer{
				Controller: controller,
				Client:     fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths),
			}
			backups := &vmBackupCheck{writer: writer, vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)}
			_, err := backups.OnVMBackupChanged("default/deleted-backup", backup)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Equal(t, []string{vmBackupComponentHealthName}, controller.Enqueued) {
				return
			}
			err = backups.Reconcile()
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
			if !assert.Equal(t, []string{vmBackupComponentHealthName, vmBackupComponentHealthName}, controller.Enqueued) {
				return
			}
			err = backups.Reconcile()
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
				assert.Contains(t, check.AffectedResources.Names, "default/remaining-backup")
				assert.NotContains(t, check.AffectedResources.Names, healthcheck.NamespacedName(backup.Namespace, backup.Name))
			} else {
				assert.Empty(t, health.Status.Checks)
			}
		})
	}
}

func TestReconcileVMBackupsKeepsSameNameInDifferentNamespaces(t *testing.T) {
	failed := []harvesterv1.VolumeBackup{{Error: &harvesterv1.Error{Message: errorMessage("backup failed")}}}
	backup := newVMBackup("backup", failed)
	other := newVMBackup("backup", failed)
	other.Namespace = "other"
	clientset, healthset := testutil.NewClients(backup, other)
	writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
	backups := &vmBackupCheck{writer: writer, vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)}

	if !assert.NoError(t, backups.Reconcile()) {
		return
	}

	health, err := healthset.HealthV1beta1().ComponentHealths().Get(context.Background(), vmBackupComponentHealthName, metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}
	check := health.Status.Checks[checkKeyVMBackupFailed]
	assert.Equal(t, 2, check.AffectedCount)
	assert.Contains(t, check.AffectedResources.Names, "default/backup")
	assert.Contains(t, check.AffectedResources.Names, "other/backup")
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
				assert.Contains(t, check.AffectedResources.Names, "default/failed-vmbackup")
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
			clientset, healthset := testutil.NewClients(tc.vmBackup)
			writer := healthcheck.Writer{Client: fakeclients.ComponentHealthClient(healthset.HealthV1beta1().ComponentHealths)}
			backups := &vmBackupCheck{writer: writer, vmBackupCache: fakeclients.VMBackupCache(clientset.HarvesterhciV1beta1().VirtualMachineBackups)}

			err := backups.Reconcile()
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
