package componenthealth

import (
	"fmt"
	"sort"
	"strings"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

// resourceRef identifies the kind of the resources a check reports on.
type resourceRef struct {
	apiVersion string
	kind       string
}

// newCheckResult builds a CheckResult whose message is "<summary>[: <messages joined by '; '>]".
func newCheckResult(severity healthv1.Severity, summary string, ref resourceRef, names []string, messages []string) healthv1.CheckResult {
	nameMap := make(map[string]healthv1.AffectedResourceDetail, len(names))
	for _, name := range names {
		nameMap[name] = healthv1.AffectedResourceDetail{}
	}

	message := summary
	if len(messages) > 0 {
		message = fmt.Sprintf("%s: %s", summary, strings.Join(messages, "; "))
	}

	return healthv1.CheckResult{
		Severity:      severity,
		Message:       message,
		AffectedCount: len(names),
		AffectedResources: &healthv1.AffectedResources{
			APIVersion: ref.apiVersion,
			Kind:       ref.kind,
			Names:      nameMap,
		},
	}
}

// resourceGroup collects the affected resource names and distinct messages of one check key.
type resourceGroup struct {
	names    []string
	messages map[string]struct{}
}

// resourceGroups groups affected resources by check key.
type resourceGroups map[string]*resourceGroup

// add records name under key; an empty message is not recorded.
func (g resourceGroups) add(key, name, message string) {
	group, ok := g[key]
	if !ok {
		group = &resourceGroup{messages: map[string]struct{}{}}
		g[key] = group
	}
	group.names = append(group.names, name)
	if message != "" {
		group.messages[message] = struct{}{}
	}
}

func getSortedMessages(messageSet map[string]struct{}) []string {
	messages := make([]string, 0, len(messageSet))
	for message := range messageSet {
		messages = append(messages, message)
	}
	sort.Strings(messages)
	return messages
}
