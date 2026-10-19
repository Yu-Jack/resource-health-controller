package componenthealth

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubevirtv1 "kubevirt.io/api/core/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

const (
	vmiControllerName = "componentHealthVMIController"
)

// vmMigrationRule is an atomic live-migration blocking condition of a VMI.
type vmMigrationRule struct {
	Key     string
	Check   func(*kubevirtv1.VirtualMachineInstance) bool
	Message string
}

var (
	ruleHasNodeSelectorHostname = vmMigrationRule{
		Key:     "VMNodeSelectorLiveMigrationUnsupported",
		Check:   hasNodeSelectorHostname,
		Message: "node selector is set",
	}
	ruleHasHostDevicesOrGPUs = vmMigrationRule{
		Key:     "VMHostDeviceLiveMigrationUnsupported",
		Check:   hasHostDevicesOrGPUs,
		Message: "PCIe, USB or vGPU devices are attached",
	}
)

// hasNodeSelectorHostname reports whether the VMI is pinned to a specific node via hostname node selector,
// which blocks live migration.
func hasNodeSelectorHostname(vmi *kubevirtv1.VirtualMachineInstance) bool {
	return vmi.Spec.NodeSelector != nil && vmi.Spec.NodeSelector[corev1.LabelHostname] != ""
}

// hasHostDevicesOrGPUs reports whether the VMI has PCIe/USB host devices or vGPU devices attached,
// either of which blocks live migration.
func hasHostDevicesOrGPUs(vmi *kubevirtv1.VirtualMachineInstance) bool {
	return len(vmi.Spec.Domain.Devices.HostDevices) != 0 || len(vmi.Spec.Domain.Devices.GPUs) != 0
}

// getVMINamesMatching returns the namespaced names of VMIs for which match returns true.
func getVMINamesMatching(vmis []*kubevirtv1.VirtualMachineInstance, match func(*kubevirtv1.VirtualMachineInstance) bool) []string {
	var names []string
	for _, vmi := range vmis {
		if match(vmi) {
			names = append(names, fmt.Sprintf("%s/%s", vmi.Namespace, vmi.Name))
		}
	}
	return names
}

// vmMigrationHealthChecks maps VM live-migration rules to the ComponentHealth checks they report.
var vmMigrationHealthChecks = []struct {
	Rule     vmMigrationRule
	Severity healthv1.Severity
}{
	{
		Rule:     ruleHasHostDevicesOrGPUs,
		Severity: healthv1.SeverityWarning,
	},
	{
		Rule:     ruleHasNodeSelectorHostname,
		Severity: healthv1.SeverityWarning,
	},
}

// OnVMIChanged queues a ComponentHealth reconciliation on every VMI change.
func (h *Handler) OnVMIChanged(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	h.enqueueHealth(vmComponentHealthName)
	return vmi, nil
}

func (h *Handler) reconcileVMI() error {
	vmis, err := h.vmiCache.List(corev1.NamespaceAll, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list VirtualMachineInstances: %w", err)
	}

	checks := map[string]healthv1.CheckResult{}
	for _, entry := range vmMigrationHealthChecks {
		names := getVMINamesMatching(vmis, entry.Rule.Check)
		if len(names) == 0 {
			continue
		}
		checks[entry.Rule.Key] = buildVMCheckResult(entry.Severity, entry.Rule.Message, names)
	}

	return h.replaceComponentHealthChecks(vmComponentHealthName, checks)
}

func buildVMCheckResult(severity healthv1.Severity, reason string, names []string) healthv1.CheckResult {
	nameMap := make(map[string]healthv1.AffectedResourceDetail, len(names))
	for _, name := range names {
		nameMap[name] = healthv1.AffectedResourceDetail{}
	}

	return healthv1.CheckResult{
		Severity:      severity,
		Message:       fmt.Sprintf("%d VM(s) cannot be live migrated: %s", len(names), reason),
		AffectedCount: len(names),
		AffectedResources: &healthv1.AffectedResources{
			APIVersion: kubevirtv1.SchemeGroupVersion.String(),
			Kind:       kubevirtv1.VirtualMachineGroupVersionKind.Kind,
			Names:      nameMap,
		},
	}
}
