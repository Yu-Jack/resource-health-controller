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
	volumeRemoteRestoreControllerName      = "componentHealthVolumeRemoteRestoreController"
	volumeRemoteRestoreComponentHealthName = "harvester-controller-volumeremoterestore"
	volumeRemoteRestoreKind                = "VolumeRemoteRestore"

	checkKeyVolumeRemoteRestoreFailed = "VolumeRemoteRestoreFailed"
)

type volumeRemoteRestoreCheck struct {
	writer healthcheck.Writer
	cache  ctlharvesterv1.VolumeRemoteRestoreCache
}

// NewVolumeRemoteRestoreCheck builds the check that reports failed VolumeRemoteRestores.
func NewVolumeRemoteRestoreCheck(writer healthcheck.Writer, cache ctlharvesterv1.VolumeRemoteRestoreCache) healthcheck.Check {
	return &volumeRemoteRestoreCheck{writer: writer, cache: cache}
}

func (c *volumeRemoteRestoreCheck) Name() string { return volumeRemoteRestoreComponentHealthName }

func (c *volumeRemoteRestoreCheck) OnVolumeRemoteRestoreChanged(_ string, obj *harvesterv1.VolumeRemoteRestore) (*harvesterv1.VolumeRemoteRestore, error) {
	c.writer.Enqueue(volumeRemoteRestoreComponentHealthName)
	return obj, nil
}

// RegisterVolumeRemoteRestoreCheck builds the VolumeRemoteRestore check with its cache and registers its change handler.
func RegisterVolumeRemoteRestoreCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	objs := management.HarvesterFactory.Harvesterhci().V1beta1().VolumeRemoteRestore()
	check := &volumeRemoteRestoreCheck{writer: writer, cache: objs.Cache()}
	objs.OnChange(ctx, volumeRemoteRestoreControllerName, check.OnVolumeRemoteRestoreChanged)
	return check, objs.Informer().HasSynced
}

func (c *volumeRemoteRestoreCheck) Reconcile() error {
	objs, err := c.cache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VolumeRemoteRestores: %w", err)
	}

	groups := healthcheck.ResourceGroups{}
	for _, obj := range objs {
		if obj.DeletionTimestamp != nil {
			continue
		}
		if volumeRemoteReady.IsFalse(obj) && volumeRemoteReady.GetMessage(obj) != volumeRemoteInProgressMessage {
			name := healthcheck.NamespacedName(obj.Namespace, obj.Name)
			groups.Add(checkKeyVolumeRemoteRestoreFailed, name, volumeRemoteRestoreMessage(obj, name))
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for key, group := range groups {
		checks[key] = healthcheck.NewCheckResult(healthcheck.CheckResultSpec{
			Severity:   healthv1.SeverityError,
			Summary:    fmt.Sprintf("%d VolumeRemoteRestore(s)", len(group.Names)),
			APIVersion: harvesterv1.SchemeGroupVersion.String(),
			Kind:       volumeRemoteRestoreKind,
			Names:      group.Names,
			Messages:   healthcheck.SortedMessages(group.Messages),
		})
	}
	return c.writer.ReplaceComponentHealthChecks(volumeRemoteRestoreComponentHealthName, checks)
}

func volumeRemoteRestoreMessage(obj *harvesterv1.VolumeRemoteRestore, name string) string {
	return fmt.Sprintf("%s: success=%t, %s=%s, reason=%q, message=%q",
		name, obj.Status.Success, volumeRemoteReady, volumeRemoteReady.GetStatus(obj), volumeRemoteReady.GetReason(obj), volumeRemoteReady.GetMessage(obj))
}
