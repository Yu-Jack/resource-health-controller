package componenthealth

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

const (
	scheduleVMBackupControllerName      = "componentHealthScheduleVMBackupController"
	scheduleVMBackupComponentHealthName = "harvester-controller-schedulevmbackup"
	scheduleVMBackupKind                = "ScheduleVMBackup"
)

// OnScheduleVMBackupChanged queues a ComponentHealth reconciliation on every ScheduleVMBackup change.
func (h *Handler) OnScheduleVMBackupChanged(_ string, scheduleVMBackup *harvesterv1.ScheduleVMBackup) (*harvesterv1.ScheduleVMBackup, error) {
	h.enqueueHealth(scheduleVMBackupComponentHealthName)
	return scheduleVMBackup, nil
}

func (h *Handler) reconcileScheduleVMBackups() error {
	scheduleVMBackups, err := h.scheduleVMBackupCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list ScheduleVMBackups: %w", err)
	}

	summariesByReason := resourceGroups{}
	for _, scheduleVMBackup := range scheduleVMBackups {
		for _, condition := range getTrueConditions(scheduleVMBackup) {
			summariesByReason.add(condition.Reason, scheduleVMBackup.Name, condition.Message)
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for reason, summary := range summariesByReason {
		checks[reason] = buildScheduleVMBackupCheckResult(reason, summary.names, getSortedMessages(summary.messages))
	}

	return h.replaceComponentHealthChecks(scheduleVMBackupComponentHealthName, checks)
}

// getTrueConditions returns the ScheduleVMBackup conditions reporting a problem, i.e. status True with a reason.
func getTrueConditions(scheduleVMBackup *harvesterv1.ScheduleVMBackup) []harvesterv1.Condition {
	conditions := make([]harvesterv1.Condition, 0)
	for _, condition := range scheduleVMBackup.Status.Conditions {
		if condition.Status == corev1.ConditionTrue && condition.Reason != "" {
			conditions = append(conditions, condition)
		}
	}
	return conditions
}

func buildScheduleVMBackupCheckResult(reason string, names []string, messages []string) healthv1.CheckResult {
	return newCheckResult(healthv1.SeverityError, fmt.Sprintf("%d ScheduleVMBackup(s) have %s", len(names), reason),
		resourceRef{apiVersion: harvesterv1.SchemeGroupVersion.String(), kind: scheduleVMBackupKind}, names, messages)
}
