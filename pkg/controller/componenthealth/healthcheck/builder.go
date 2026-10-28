package healthcheck

import (
	"fmt"
	"sort"
	"strings"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

// NamespacedName is the name under which a namespaced resource is reported in AffectedResources.
func NamespacedName(namespace, name string) string {
	return namespace + "/" + name
}

// CheckResultSpec describes a CheckResult; Messages is optional.
type CheckResultSpec struct {
	Severity   healthv1.Severity
	Summary    string
	APIVersion string
	Kind       string
	Names      []string
	Messages   []string
}

// NewCheckResult builds a CheckResult whose message is "<summary>[: <messages joined by '; '>]".
func NewCheckResult(spec CheckResultSpec) healthv1.CheckResult {
	nameMap := make(map[string]healthv1.AffectedResourceDetail, len(spec.Names))
	for _, name := range spec.Names {
		nameMap[name] = healthv1.AffectedResourceDetail{}
	}

	message := spec.Summary
	if len(spec.Messages) > 0 {
		message = fmt.Sprintf("%s: %s", spec.Summary, strings.Join(spec.Messages, "; "))
	}

	return healthv1.CheckResult{
		Severity:      spec.Severity,
		Message:       message,
		AffectedCount: len(spec.Names),
		AffectedResources: &healthv1.AffectedResources{
			APIVersion: spec.APIVersion,
			Kind:       spec.Kind,
			Names:      nameMap,
		},
	}
}

// ResourceGroup collects the affected resource names and distinct messages of one check key.
type ResourceGroup struct {
	Names    []string
	Messages map[string]struct{}
}

// ResourceGroups groups affected resources by check key.
type ResourceGroups map[string]*ResourceGroup

// Add records name under key; an empty message is not recorded.
func (g ResourceGroups) Add(key, name, message string) {
	group, ok := g[key]
	if !ok {
		group = &ResourceGroup{Messages: map[string]struct{}{}}
		g[key] = group
	}
	group.Names = append(group.Names, name)
	if message != "" {
		group.Messages[message] = struct{}{}
	}
}

// SortedMessages returns the messages of a set in a deterministic order.
func SortedMessages(messageSet map[string]struct{}) []string {
	messages := make([]string, 0, len(messageSet))
	for message := range messageSet {
		messages = append(messages, message)
	}
	sort.Strings(messages)
	return messages
}
