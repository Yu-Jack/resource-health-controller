package componenthealth

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

const (
	nodeControllerName   = "componentHealthNodeController"
	checkKeyNodeCordoned = "NodeCordoned"
	nodeKind             = "Node"
)

func (h *Handler) OnNodeChanged(_ string, node *corev1.Node) (*corev1.Node, error) {
	h.enqueueHealth(nodeComponentHealthName)
	return node, nil
}

func (h *Handler) reconcileNodes() error {
	nodes, err := h.nodeCache.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list Nodes: %w", err)
	}

	cordonedNodeNames := make([]string, 0)
	for _, node := range nodes {
		if node.Spec.Unschedulable {
			cordonedNodeNames = append(cordonedNodeNames, node.Name)
		}
	}

	checks := map[string]healthv1.CheckResult{}
	if len(cordonedNodeNames) > 0 {
		checks[checkKeyNodeCordoned] = buildNodeCordonedCheckResult(cordonedNodeNames)
	}

	return h.replaceComponentHealthChecks(nodeComponentHealthName, checks)
}

func buildNodeCordonedCheckResult(names []string) healthv1.CheckResult {
	nameMap := make(map[string]healthv1.AffectedResourceDetail, len(names))
	for _, name := range names {
		nameMap[name] = healthv1.AffectedResourceDetail{}
	}

	return healthv1.CheckResult{
		Severity:      healthv1.SeverityInfo,
		Message:       fmt.Sprintf("%d Node(s) are cordoned", len(names)),
		AffectedCount: len(names),
		AffectedResources: &healthv1.AffectedResources{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       nodeKind,
			Names:      nameMap,
		},
	}
}
