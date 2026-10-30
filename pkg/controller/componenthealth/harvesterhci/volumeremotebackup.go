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
	volumeRemoteBackupControllerName      = "componentHealthVolumeRemoteBackupController"
	volumeRemoteBackupComponentHealthName = "harvester-controller-volumeremotebackup"
	volumeRemoteBackupKind                = "VolumeRemoteBackup"

	checkKeyVolumeRemoteBackupFailed = "VolumeRemoteBackupFailed"
)

type volumeRemoteBackupCheck struct {
	writer healthcheck.Writer
	cache  ctlharvesterv1.VolumeRemoteBackupCache
}

// NewVolumeRemoteBackupCheck builds the check that reports failed VolumeRemoteBackups.
func NewVolumeRemoteBackupCheck(writer healthcheck.Writer, cache ctlharvesterv1.VolumeRemoteBackupCache) healthcheck.Check {
	return &volumeRemoteBackupCheck{writer: writer, cache: cache}
}

func (c *volumeRemoteBackupCheck) Name() string { return volumeRemoteBackupComponentHealthName }

func (c *volumeRemoteBackupCheck) OnVolumeRemoteBackupChanged(_ string, obj *harvesterv1.VolumeRemoteBackup) (*harvesterv1.VolumeRemoteBackup, error) {
	c.writer.Enqueue(volumeRemoteBackupComponentHealthName)
	return obj, nil
}

// RegisterVolumeRemoteBackupCheck builds the VolumeRemoteBackup check with its cache and registers its change handler.
func RegisterVolumeRemoteBackupCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	objs := management.HarvesterFactory.Harvesterhci().V1beta1().VolumeRemoteBackup()
	check := &volumeRemoteBackupCheck{writer: writer, cache: objs.Cache()}
	objs.OnChange(ctx, volumeRemoteBackupControllerName, check.OnVolumeRemoteBackupChanged)
	return check, objs.Informer().HasSynced
}

func (c *volumeRemoteBackupCheck) Reconcile() error {
	objs, err := c.cache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VolumeRemoteBackups: %w", err)
	}

	groups := healthcheck.ResourceGroups{}
	for _, obj := range objs {
		if obj.DeletionTimestamp != nil {
			continue
		}
		if volumeRemoteReady.IsFalse(obj) && volumeRemoteReady.GetMessage(obj) != volumeRemoteInProgressMessage {
			name := healthcheck.NamespacedName(obj.Namespace, obj.Name)
			groups.Add(checkKeyVolumeRemoteBackupFailed, name, volumeRemoteBackupMessage(obj, name))
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for key, group := range groups {
		checks[key] = healthcheck.NewCheckResult(healthcheck.CheckResultSpec{
			Severity:   healthv1.SeverityError,
			Summary:    fmt.Sprintf("%d VolumeRemoteBackup(s)", len(group.Names)),
			APIVersion: harvesterv1.SchemeGroupVersion.String(),
			Kind:       volumeRemoteBackupKind,
			Names:      group.Names,
			Messages:   healthcheck.SortedMessages(group.Messages),
		})
	}
	return c.writer.ReplaceComponentHealthChecks(volumeRemoteBackupComponentHealthName, checks)
}

func volumeRemoteBackupMessage(obj *harvesterv1.VolumeRemoteBackup, name string) string {
	return fmt.Sprintf("%s: success=%t, %s=%s, reason=%q, message=%q",
		name, obj.Status.Success, volumeRemoteReady, volumeRemoteReady.GetStatus(obj), volumeRemoteReady.GetReason(obj), volumeRemoteReady.GetMessage(obj))
}
