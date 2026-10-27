package componenthealth

import (
	"testing"

	"github.com/stretchr/testify/assert"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

func TestNamespacedName(t *testing.T) {
	assert.Equal(t, "default/vm", namespacedName("default", "vm"))
}

func TestNewCheckResult(t *testing.T) {
	ref := resourceRef{apiVersion: "example.io/v1", kind: "Example"}

	t.Run("messages are appended to the summary", func(t *testing.T) {
		check := newCheckResult(healthv1.SeverityWarning, "2 Example(s) failed", ref, []string{"a", "b"}, []string{"first", "second"})

		assert.Equal(t, healthv1.SeverityWarning, check.Severity)
		assert.Equal(t, "2 Example(s) failed: first; second", check.Message)
		assert.Equal(t, 2, check.AffectedCount)
		assert.Equal(t, "example.io/v1", check.AffectedResources.APIVersion)
		assert.Equal(t, "Example", check.AffectedResources.Kind)
		assert.Equal(t, map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}}, check.AffectedResources.Names)
	})

	t.Run("summary is the message when there are no messages", func(t *testing.T) {
		check := newCheckResult(healthv1.SeverityInfo, "1 Example(s) are cordoned", ref, []string{"a"}, nil)

		assert.Equal(t, "1 Example(s) are cordoned", check.Message)
	})

	t.Run("no names gives an empty set", func(t *testing.T) {
		check := newCheckResult(healthv1.SeverityInfo, "none", ref, nil, nil)

		assert.Equal(t, 0, check.AffectedCount)
		assert.NotNil(t, check.AffectedResources.Names)
		assert.Empty(t, check.AffectedResources.Names)
	})
}

func TestResourceGroupsAdd(t *testing.T) {
	groups := resourceGroups{}
	groups.add("reasonA", "a", "same message")
	groups.add("reasonA", "b", "same message")
	groups.add("reasonA", "c", "")
	groups.add("reasonB", "d", "other message")

	assert.Len(t, groups, 2)
	assert.Equal(t, []string{"a", "b", "c"}, groups["reasonA"].names)
	assert.Equal(t, map[string]struct{}{"same message": {}}, groups["reasonA"].messages)
	assert.Equal(t, []string{"d"}, groups["reasonB"].names)
	assert.Equal(t, map[string]struct{}{"other message": {}}, groups["reasonB"].messages)
}

func TestGetSortedMessages(t *testing.T) {
	messages := getSortedMessages(map[string]struct{}{"b": {}, "c": {}, "a": {}})

	assert.Equal(t, []string{"a", "b", "c"}, messages)
	assert.Empty(t, getSortedMessages(nil))
}
