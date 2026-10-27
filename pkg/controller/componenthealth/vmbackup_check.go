package componenthealth

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

const (
	vmBackupControllerName      = "componentHealthVMBackupController"
	vmBackupComponentHealthName = "harvester-controller-vmbackup"
	vmBackupKind                = "VirtualMachineBackup"

	// checkKeyVMBackupFailed is the ComponentHealth check key reported when a VirtualMachineBackup's
	// latest VolumeBackup fails.
	checkKeyVMBackupFailed = "VMBackupFailed"
)

// OnVMBackupChanged queues a ComponentHealth reconciliation on every VirtualMachineBackup change.
func (h *Handler) OnVMBackupChanged(_ string, vmBackup *harvesterv1.VirtualMachineBackup) (*harvesterv1.VirtualMachineBackup, error) {
	h.enqueueHealth(vmBackupComponentHealthName)
	return vmBackup, nil
}

func (h *Handler) reconcileVMBackups() error {
	vmBackups, err := h.vmBackupCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VirtualMachineBackups: %w", err)
	}

	var names []string
	messageSet := map[string]struct{}{}
	for _, vmBackup := range vmBackups {
		volumeBackup := getLatestVolumeBackupWithError(vmBackup)
		if volumeBackup == nil {
			continue
		}
		names = append(names, vmBackup.Name)
		messageSet[*volumeBackup.Error.Message] = struct{}{}
	}

	checks := map[string]healthv1.CheckResult{}
	if len(names) > 0 {
		checks[checkKeyVMBackupFailed] = buildVMBackupCheckResult(names, getSortedMessages(messageSet))
	}

	return h.replaceComponentHealthChecks(vmBackupComponentHealthName, checks)
}

// getLatestVolumeBackupWithError returns the most recently created VolumeBackup that has an
// error, or nil if the VirtualMachineBackup has no failed VolumeBackup.
func getLatestVolumeBackupWithError(vmBackup *harvesterv1.VirtualMachineBackup) *harvesterv1.VolumeBackup {
	var latest *harvesterv1.VolumeBackup
	for i := range vmBackup.Status.VolumeBackups {
		volumeBackup := &vmBackup.Status.VolumeBackups[i]
		if volumeBackup.Error == nil || volumeBackup.Error.Message == nil {
			continue
		}
		if latest == nil || volumeBackupCreatedAfter(volumeBackup, latest) {
			latest = volumeBackup
		}
	}
	return latest
}

func volumeBackupCreatedAfter(a, b *harvesterv1.VolumeBackup) bool {
	if a.CreationTime == nil {
		return false
	}
	if b.CreationTime == nil {
		return true
	}
	return a.CreationTime.After(b.CreationTime.Time)
}

func buildVMBackupCheckResult(names []string, messages []string) healthv1.CheckResult {
	return newCheckResult(healthv1.SeverityError, fmt.Sprintf("%d VirtualMachineBackup(s) failed", len(names)),
		resourceRef{apiVersion: harvesterv1.SchemeGroupVersion.String(), kind: vmBackupKind}, names, messages)
}
