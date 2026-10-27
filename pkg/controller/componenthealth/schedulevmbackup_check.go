package componenthealth

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	ctlharvesterv1 "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
)

const (
	scheduleVMBackupControllerName      = "componentHealthScheduleVMBackupController"
	scheduleVMBackupComponentHealthName = "harvester-controller-schedulevmbackup"
	scheduleVMBackupKind                = "ScheduleVMBackup"
)

type scheduleVMBackupCheck struct {
	writer                componentHealthWriter
	scheduleVMBackupCache ctlharvesterv1.ScheduleVMBackupCache
}

func (c *scheduleVMBackupCheck) name() string { return scheduleVMBackupComponentHealthName }

// OnScheduleVMBackupChanged queues a ComponentHealth reconciliation on every ScheduleVMBackup change.
func (c *scheduleVMBackupCheck) OnScheduleVMBackupChanged(_ string, scheduleVMBackup *harvesterv1.ScheduleVMBackup) (*harvesterv1.ScheduleVMBackup, error) {
	c.writer.enqueue(scheduleVMBackupComponentHealthName)
	return scheduleVMBackup, nil
}

func registerScheduleVMBackupCheck(ctx context.Context, writer componentHealthWriter, management *config.Management) (healthCheck, cache.InformerSynced) {
	scheduleVMBackups := management.HarvesterFactory.Harvesterhci().V1beta1().ScheduleVMBackup()
	check := &scheduleVMBackupCheck{writer: writer, scheduleVMBackupCache: scheduleVMBackups.Cache()}
	scheduleVMBackups.OnChange(ctx, scheduleVMBackupControllerName, check.OnScheduleVMBackupChanged)
	return check, scheduleVMBackups.Informer().HasSynced
}

func (c *scheduleVMBackupCheck) reconcile() error {
	scheduleVMBackups, err := c.scheduleVMBackupCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list ScheduleVMBackups: %w", err)
	}

	summariesByReason := resourceGroups{}
	for _, scheduleVMBackup := range scheduleVMBackups {
		for _, condition := range getTrueConditions(scheduleVMBackup) {
			summariesByReason.add(condition.Reason, namespacedName(scheduleVMBackup.Namespace, scheduleVMBackup.Name), condition.Message)
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for reason, summary := range summariesByReason {
		checks[reason] = buildScheduleVMBackupCheckResult(reason, summary.names, getSortedMessages(summary.messages))
	}

	return c.writer.replaceComponentHealthChecks(scheduleVMBackupComponentHealthName, checks)
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
