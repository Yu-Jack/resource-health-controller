package harvesterhci

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
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
)

const (
	scheduleVMBackupControllerName      = "componentHealthScheduleVMBackupController"
	scheduleVMBackupComponentHealthName = "harvester-controller-schedulevmbackup"
	scheduleVMBackupKind                = "ScheduleVMBackup"
)

type scheduleVMBackupCheck struct {
	writer                healthcheck.Writer
	scheduleVMBackupCache ctlharvesterv1.ScheduleVMBackupCache
}

// NewScheduleVMBackupCheck builds the check that reports ScheduleVMBackups with a problem condition.
func NewScheduleVMBackupCheck(writer healthcheck.Writer, scheduleVMBackupCache ctlharvesterv1.ScheduleVMBackupCache) healthcheck.Check {
	return &scheduleVMBackupCheck{writer: writer, scheduleVMBackupCache: scheduleVMBackupCache}
}

func (c *scheduleVMBackupCheck) Name() string { return scheduleVMBackupComponentHealthName }

// OnScheduleVMBackupChanged queues a ComponentHealth reconciliation on every ScheduleVMBackup change.
func (c *scheduleVMBackupCheck) OnScheduleVMBackupChanged(_ string, scheduleVMBackup *harvesterv1.ScheduleVMBackup) (*harvesterv1.ScheduleVMBackup, error) {
	c.writer.Enqueue(scheduleVMBackupComponentHealthName)
	return scheduleVMBackup, nil
}

// RegisterScheduleVMBackupCheck builds the ScheduleVMBackup check with its cache and registers its change handler.
func RegisterScheduleVMBackupCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	scheduleVMBackups := management.HarvesterFactory.Harvesterhci().V1beta1().ScheduleVMBackup()
	check := &scheduleVMBackupCheck{writer: writer, scheduleVMBackupCache: scheduleVMBackups.Cache()}
	scheduleVMBackups.OnChange(ctx, scheduleVMBackupControllerName, check.OnScheduleVMBackupChanged)
	return check, scheduleVMBackups.Informer().HasSynced
}

func (c *scheduleVMBackupCheck) Reconcile() error {
	scheduleVMBackups, err := c.scheduleVMBackupCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list ScheduleVMBackups: %w", err)
	}

	summariesByReason := healthcheck.ResourceGroups{}
	for _, scheduleVMBackup := range scheduleVMBackups {
		for _, condition := range getTrueConditions(scheduleVMBackup) {
			summariesByReason.Add(condition.Reason, healthcheck.NamespacedName(scheduleVMBackup.Namespace, scheduleVMBackup.Name), condition.Message)
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for reason, summary := range summariesByReason {
		checks[reason] = buildScheduleVMBackupCheckResult(reason, summary.Names, healthcheck.SortedMessages(summary.Messages))
	}

	return c.writer.ReplaceComponentHealthChecks(scheduleVMBackupComponentHealthName, checks)
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
	return healthcheck.NewCheckResult(healthv1.SeverityError, fmt.Sprintf("%d ScheduleVMBackup(s) have %s", len(names), reason),
		healthcheck.ResourceRef{APIVersion: harvesterv1.SchemeGroupVersion.String(), Kind: scheduleVMBackupKind}, names, messages)
}
