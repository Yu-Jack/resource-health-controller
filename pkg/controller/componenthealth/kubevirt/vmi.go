// Package kubevirt holds the checks on the kubevirt.io API group.
package kubevirt

import (
	"context"
	"fmt"

	ctlkubevirtv1 "github.com/harvester/harvester/pkg/generated/controllers/kubevirt.io/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	kubevirtv1 "kubevirt.io/api/core/v1"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
)

const (
	vmiControllerName     = "componentHealthVMIController"
	vmComponentHealthName = "harvester-controller-vm"
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
			names = append(names, healthcheck.NamespacedName(vmi.Namespace, vmi.Name))
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

type vmiCheck struct {
	writer   healthcheck.Writer
	vmiCache ctlkubevirtv1.VirtualMachineInstanceCache
}

// NewVMICheck builds the check that reports VMIs that cannot be live migrated.
func NewVMICheck(writer healthcheck.Writer, vmiCache ctlkubevirtv1.VirtualMachineInstanceCache) healthcheck.Check {
	return &vmiCheck{writer: writer, vmiCache: vmiCache}
}

func (c *vmiCheck) Name() string { return vmComponentHealthName }

// OnVMIChanged queues a ComponentHealth reconciliation on every VMI change.
func (c *vmiCheck) OnVMIChanged(_ string, vmi *kubevirtv1.VirtualMachineInstance) (*kubevirtv1.VirtualMachineInstance, error) {
	c.writer.Enqueue(vmComponentHealthName)
	return vmi, nil
}

// RegisterVMICheck builds the VMI check with its cache and registers its change handler.
func RegisterVMICheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	vmis := management.KubeVirtFactory.Kubevirt().V1().VirtualMachineInstance()
	check := &vmiCheck{writer: writer, vmiCache: vmis.Cache()}
	vmis.OnChange(ctx, vmiControllerName, check.OnVMIChanged)
	return check, vmis.Informer().HasSynced
}

func (c *vmiCheck) Reconcile() error {
	vmis, err := c.vmiCache.List(corev1.NamespaceAll, labels.Everything())
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

	return c.writer.ReplaceComponentHealthChecks(vmComponentHealthName, checks)
}

func buildVMCheckResult(severity healthv1.Severity, reason string, names []string) healthv1.CheckResult {
	return healthcheck.NewCheckResult(healthcheck.CheckResultSpec{
		Severity:   severity,
		Summary:    fmt.Sprintf("%d VM(s) cannot be live migrated", len(names)),
		APIVersion: kubevirtv1.SchemeGroupVersion.String(),
		Kind:       kubevirtv1.VirtualMachineGroupVersionKind.Kind,
		Names:      names,
		Messages:   []string{reason},
	})
}
