package componenthealth

import (
	"context"
	"fmt"
	"strings"

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

	checks := map[string]healthv1.CheckResult{}
	messages := map[string]map[string]struct{}{}
	for _, image := range images {
		if image.DeletionTimestamp != nil {
			continue
		}
		key, severity, cond := vmImageFailure(image)
		if key != "" {
			addVMImageCheck(checks, messages, image, key, severity, cond)
		}
		if harvesterv1.BackingImageMissing.IsTrue(image) {
			addVMImageCheck(checks, messages, image, checkKeyImageBackingImageMissing, healthv1.SeverityWarning, harvesterv1.BackingImageMissing)
		}
	}
	for key, check := range checks {
		check.Message = fmt.Sprintf("%d VirtualMachineImage(s): %s", check.AffectedCount, strings.Join(getSortedMessages(messages[key]), "; "))
		checks[key] = check
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

func addVMImageCheck(checks map[string]healthv1.CheckResult, messages map[string]map[string]struct{}, image *harvesterv1.VirtualMachineImage, key string, severity healthv1.Severity, cond condition.Cond) {
	check, ok := checks[key]
	if !ok {
		check = healthv1.CheckResult{
			Severity: severity,
			AffectedResources: &healthv1.AffectedResources{
				APIVersion: harvesterv1.SchemeGroupVersion.String(),
				Kind:       vmImageKind,
				Names:      map[string]healthv1.AffectedResourceDetail{},
			},
		}
		messages[key] = map[string]struct{}{}
	}
	name := image.Namespace + "/" + image.Name
	check.AffectedResources.Names[name] = healthv1.AffectedResourceDetail{}
	check.AffectedCount = len(check.AffectedResources.Names)
	message := fmt.Sprintf("%s: %s=%s, reason=%q, message=%q, failed=%d, retry=%d, lastFailedTime=%q",
		name, cond, cond.GetStatus(image), cond.GetReason(image), cond.GetMessage(image),
		image.Status.Failed, image.Spec.Retry, image.Status.LastFailedTime)
	if cond == harvesterv1.BackingImageMissing {
		message = "previous backing image lookup failed (condition is not automatically cleared): " + message
	}
	messages[key][message] = struct{}{}
	checks[key] = check
}
