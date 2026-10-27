package componenthealth

import (
	"context"
	"fmt"

	"github.com/rancher/wrangler/v3/pkg/condition"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	harvesterv1 "github.com/harvester/harvester/pkg/apis/harvesterhci.io/v1beta1"
	ctlharvesterv1 "github.com/harvester/harvester/pkg/generated/controllers/harvesterhci.io/v1beta1"
	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
)

const (
	vmImageControllerName      = "componentHealthVMImageController"
	vmImageComponentHealthName = "harvester-controller-vmimage"
	vmImageKind                = "VirtualMachineImage"

	checkKeyImageRetryLimitExceeded   = "ImageRetryLimitExceeded"
	checkKeyImageUploadFailed         = "ImageUploadFailed"
	checkKeyImageImportFailed         = "ImageImportFailed"
	checkKeyImageInitializationFailed = "ImageInitializationFailed"
	checkKeyImageRetryingAfterFailure = "ImageRetryingAfterFailure"
	checkKeyImageBackingImageMissing  = "ImageBackingImageMissing"
)

type vmImageCheck struct {
	writer       componentHealthWriter
	vmImageCache ctlharvesterv1.VirtualMachineImageCache
}

func (c *vmImageCheck) name() string { return vmImageComponentHealthName }

func (c *vmImageCheck) OnVMImageChanged(_ string, image *harvesterv1.VirtualMachineImage) (*harvesterv1.VirtualMachineImage, error) {
	c.writer.enqueue(vmImageComponentHealthName)
	return image, nil
}

func registerVMImageCheck(ctx context.Context, writer componentHealthWriter, management *config.Management) (healthCheck, cache.InformerSynced) {
	vmImages := management.HarvesterFactory.Harvesterhci().V1beta1().VirtualMachineImage()
	check := &vmImageCheck{writer: writer, vmImageCache: vmImages.Cache()}
	vmImages.OnChange(ctx, vmImageControllerName, check.OnVMImageChanged)
	return check, vmImages.Informer().HasSynced
}

func (c *vmImageCheck) reconcile() error {
	images, err := c.vmImageCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VirtualMachineImages: %w", err)
	}

	groups := resourceGroups{}
	severities := map[string]healthv1.Severity{}
	for _, image := range images {
		if image.DeletionTimestamp != nil {
			continue
		}
		name := namespacedName(image.Namespace, image.Name)
		if key, severity, cond := vmImageFailure(image); key != "" {
			groups.add(key, name, vmImageMessage(image, name, cond))
			severities[key] = severity
		}
		if harvesterv1.BackingImageMissing.IsTrue(image) {
			groups.add(checkKeyImageBackingImageMissing, name, vmImageMessage(image, name, harvesterv1.BackingImageMissing))
			severities[checkKeyImageBackingImageMissing] = healthv1.SeverityWarning
		}
	}

	checks := map[string]healthv1.CheckResult{}
	for key, group := range groups {
		checks[key] = newCheckResult(severities[key], fmt.Sprintf("%d VirtualMachineImage(s)", len(group.names)),
			resourceRef{apiVersion: harvesterv1.SchemeGroupVersion.String(), kind: vmImageKind}, group.names, getSortedMessages(group.messages))
	}
	return c.writer.replaceComponentHealthChecks(vmImageComponentHealthName, checks)
}

func vmImageFailure(image *harvesterv1.VirtualMachineImage) (string, healthv1.Severity, condition.Cond) {
	if harvesterv1.ImageImported.IsTrue(image) {
		return "", "", ""
	}
	switch {
	case harvesterv1.ImageRetryLimitExceeded.IsTrue(image):
		return checkKeyImageRetryLimitExceeded, healthv1.SeverityError, harvesterv1.ImageRetryLimitExceeded
	case harvesterv1.ImageImported.IsFalse(image):
		if harvesterv1.ImageImported.GetReason(image) == "UploadFailed" {
			return checkKeyImageUploadFailed, healthv1.SeverityError, harvesterv1.ImageImported
		}
		return checkKeyImageImportFailed, healthv1.SeverityError, harvesterv1.ImageImported
	case harvesterv1.ImageInitialized.IsFalse(image):
		return checkKeyImageInitializationFailed, healthv1.SeverityError, harvesterv1.ImageInitialized
	case harvesterv1.ImageRetryLimitExceeded.IsFalse(image) && harvesterv1.ImageRetryLimitExceeded.GetMessage(image) != "":
		return checkKeyImageRetryingAfterFailure, healthv1.SeverityWarning, harvesterv1.ImageRetryLimitExceeded
	default:
		return "", "", ""
	}
}

func vmImageMessage(image *harvesterv1.VirtualMachineImage, name string, cond condition.Cond) string {
	message := fmt.Sprintf("%s: %s=%s, reason=%q, message=%q, failed=%d, retry=%d, lastFailedTime=%q",
		name, cond, cond.GetStatus(image), cond.GetReason(image), cond.GetMessage(image),
		image.Status.Failed, image.Spec.Retry, image.Status.LastFailedTime)
	if cond == harvesterv1.BackingImageMissing {
		message = "previous backing image lookup failed (condition is not automatically cleared): " + message
	}
	return message
}
