package componenthealth

import (
	"fmt"

	longhornv1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta2"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

const (
	volumeControllerName = "componentHealthLonghornVolumeController"
	volumeKind           = "Volume"

	longhornNamespace = "longhorn-system"
)

// OnVolumeChanged queues a ComponentHealth reconciliation on every Longhorn Volume change.
func (h *Handler) OnVolumeChanged(_ string, volume *longhornv1.Volume) (*longhornv1.Volume, error) {
	h.enqueueHealth(volumeComponentHealthName)
	return volume, nil
}

func (h *Handler) reconcileVolumes() error {
	volumes, err := h.volumeCache.List(longhornNamespace, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list Longhorn Volumes: %w", err)
	}

	volumeSummariesByReason := resourceGroups{}
	for _, volume := range volumes {
		for _, condition := range getFalseConditions(volume) {
			logrus.WithFields(logrus.Fields{
				"volume":            volume.Name,
				"condition_type":    condition.Type,
				"condition_reason":  condition.Reason,
				"condition_message": condition.Message,
			}).Debug("found Longhorn Volume false condition")

			volumeSummariesByReason.add(condition.Reason, volume.Name, condition.Message)
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for reason, summary := range volumeSummariesByReason {
		checks[reason] = buildVolumeCheckResult(healthv1.SeverityWarning, reason, summary.names, getSortedMessages(summary.messages))
	}

	return h.replaceComponentHealthChecks(volumeComponentHealthName, checks)
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
	return newCheckResult(severity, fmt.Sprintf("%d Longhorn Volume(s) have %s", len(names), reason),
		resourceRef{apiVersion: longhornv1.SchemeGroupVersion.String(), kind: volumeKind}, names, messages)
}
