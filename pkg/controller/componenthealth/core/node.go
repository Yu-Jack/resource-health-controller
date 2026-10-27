// Package core holds the checks on the Kubernetes core API group.
package core

import (
	"context"
	"fmt"

	ctlharvcorev1 "github.com/harvester/harvester/pkg/generated/controllers/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
	"github.com/harvester/resource-health-controller/pkg/config"
	"github.com/harvester/resource-health-controller/pkg/controller/componenthealth/healthcheck"
)

const (
	nodeControllerName      = "componentHealthNodeController"
	nodeComponentHealthName = "harvester-controller-node"
	checkKeyNodeCordoned    = "NodeCordoned"
	nodeKind                = "Node"
)

type nodeCheck struct {
	writer    healthcheck.Writer
	nodeCache ctlharvcorev1.NodeCache
}

// NewNodeCheck builds the check that reports cordoned Nodes.
func NewNodeCheck(writer healthcheck.Writer, nodeCache ctlharvcorev1.NodeCache) healthcheck.Check {
	return &nodeCheck{writer: writer, nodeCache: nodeCache}
}

func (c *nodeCheck) Name() string { return nodeComponentHealthName }

func (c *nodeCheck) OnNodeChanged(_ string, node *corev1.Node) (*corev1.Node, error) {
	c.writer.Enqueue(nodeComponentHealthName)
	return node, nil
}

// RegisterNodeCheck builds the Node check with its cache and registers its change handler.
func RegisterNodeCheck(ctx context.Context, writer healthcheck.Writer, management *config.Management) (healthcheck.Check, cache.InformerSynced) {
	nodes := management.CoreFactory.Core().V1().Node()
	check := &nodeCheck{writer: writer, nodeCache: nodes.Cache()}
	nodes.OnChange(ctx, nodeControllerName, check.OnNodeChanged)
	return check, nodes.Informer().HasSynced
}

func (c *nodeCheck) Reconcile() error {
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

	return c.writer.ReplaceComponentHealthChecks(nodeComponentHealthName, checks)
}

func buildNodeCordonedCheckResult(names []string) healthv1.CheckResult {
	return healthcheck.NewCheckResult(healthv1.SeverityInfo, fmt.Sprintf("%d Node(s) are cordoned", len(names)),
		healthcheck.ResourceRef{APIVersion: corev1.SchemeGroupVersion.String(), Kind: nodeKind}, names, nil)
}
