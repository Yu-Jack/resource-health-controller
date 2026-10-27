// Package longhorn holds the checks on the longhorn.io API group.
package longhorn

import (
	"context"
	"fmt"

	ctllonghornv1 "github.com/harvester/harvester/pkg/generated/controllers/longhorn.io/v1beta2"
	longhornv1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
)

const (
	volumeControllerName      = "componentHealthLonghornVolumeController"
	volumeComponentHealthName = "harvester-controller-volume"
	volumeKind                = "Volume"

	longhornNamespace = "longhorn-system"
)

type volumeCheck struct {
	writer      healthcheck.Writer
	volumeCache ctllonghornv1.VolumeCache
}

// NewVolumeCheck builds the check that reports Longhorn Volumes with a false condition.
func NewVolumeCheck(writer healthcheck.Writer, volumeCache ctllonghornv1.VolumeCache) healthcheck.Check {
	return &volumeCheck{writer: writer, volumeCache: volumeCache}
}

func (c *volumeCheck) Name() string { return volumeComponentHealthName }

// OnVolumeChanged queues a ComponentHealth reconciliation on every Longhorn Volume change.
func (c *volumeCheck) OnVolumeChanged(_ string, volume *longhornv1.Volume) (*longhornv1.Volume, error) {
	c.writer.Enqueue(volumeComponentHealthName)
	return volume, nil
}

// RegisterVolumeCheck builds the Volume check with its cache and registers its change handler.
func RegisterVolumeCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	volumes := management.LonghornFactory.Longhorn().V1beta2().Volume()
	check := &volumeCheck{writer: writer, volumeCache: volumes.Cache()}
	volumes.OnChange(ctx, volumeControllerName, check.OnVolumeChanged)
	return check, volumes.Informer().HasSynced
}

func (c *volumeCheck) Reconcile() error {
	volumes, err := c.volumeCache.List(longhornNamespace, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list Longhorn Volumes: %w", err)
	}

	volumeSummariesByReason := healthcheck.ResourceGroups{}
	for _, volume := range volumes {
		for _, condition := range getFalseConditions(volume) {
			logrus.WithFields(logrus.Fields{
				"volume":            volume.Name,
				"condition_type":    condition.Type,
				"condition_reason":  condition.Reason,
				"condition_message": condition.Message,
			}).Debug("found Longhorn Volume false condition")

			volumeSummariesByReason.Add(condition.Reason, volume.Name, condition.Message)
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for reason, summary := range volumeSummariesByReason {
		checks[reason] = buildVolumeCheckResult(healthv1.SeverityWarning, reason, summary.Names, healthcheck.SortedMessages(summary.Messages))
	}

	return c.writer.ReplaceComponentHealthChecks(volumeComponentHealthName, checks)
}

func getFalseConditions(volume *longhornv1.Volume) []longhornv1.Condition {
	conditions := make([]longhornv1.Condition, 0)
	for _, condition := range volume.Status.Conditions {
		if condition.Status == longhornv1.ConditionStatusFalse &&
			condition.Reason != "" {
			conditions = append(conditions, condition)
		}
	}
	return conditions
}

func buildVolumeCheckResult(severity healthv1.Severity, reason string, names []string, messages []string) healthv1.CheckResult {
	return healthcheck.NewCheckResult(severity, fmt.Sprintf("%d Longhorn Volume(s) have %s", len(names), reason),
		healthcheck.ResourceRef{APIVersion: longhornv1.SchemeGroupVersion.String(), Kind: volumeKind}, names, messages)
}
