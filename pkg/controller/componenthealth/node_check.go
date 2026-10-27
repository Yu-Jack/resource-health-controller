package componenthealth

import (
	"context"
	"fmt"

	ctlharvcorev1 "github.com/harvester/harvester/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
)

const (
	nodeControllerName      = "componentHealthNodeController"
	nodeComponentHealthName = "harvester-controller-node"
	checkKeyNodeCordoned    = "NodeCordoned"
	nodeKind                = "Node"
)

type nodeCheck struct {
	writer    componentHealthWriter
	nodeCache ctlharvcorev1.NodeCache
}

func (c *nodeCheck) name() string { return nodeComponentHealthName }

func (c *nodeCheck) OnNodeChanged(_ string, node *corev1.Node) (*corev1.Node, error) {
	c.writer.enqueue(nodeComponentHealthName)
	return node, nil
}

func registerNodeCheck(ctx context.Context, writer componentHealthWriter, management *config.Management) (healthCheck, cache.InformerSynced) {
	nodes := management.CoreFactory.Core().V1().Node()
	check := &nodeCheck{writer: writer, nodeCache: nodes.Cache()}
	nodes.OnChange(ctx, nodeControllerName, check.OnNodeChanged)
	return check, nodes.Informer().HasSynced
}

func (c *nodeCheck) reconcile() error {
	nodes, err := c.nodeCache.List(labels.Everything())
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

	return c.writer.replaceComponentHealthChecks(nodeComponentHealthName, checks)
}

func buildNodeCordonedCheckResult(names []string) healthv1.CheckResult {
	return newCheckResult(healthv1.SeverityInfo, fmt.Sprintf("%d Node(s) are cordoned", len(names)),
		resourceRef{apiVersion: corev1.SchemeGroupVersion.String(), kind: nodeKind}, names, nil)
}
