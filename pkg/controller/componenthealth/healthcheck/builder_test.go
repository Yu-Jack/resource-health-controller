package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"

	healthv1 "github.com/harvester/resource-health-controller/pkg/apis/health.harvesterhci.io/v1beta1"
)

func TestNamespacedName(t *testing.T) {
	assert.Equal(t, "default/vm", NamespacedName("default", "vm"))
}

func TestNewCheckResult(t *testing.T) {
	newSpec := func(severity healthv1.Severity, summary string) CheckResultSpec {
		return CheckResultSpec{Severity: severity, Summary: summary, APIVersion: "example.io/v1", Kind: "Example"}
	}

	t.Run("messages are appended to the summary", func(t *testing.T) {
		spec := newSpec(healthv1.SeverityWarning, "2 Example(s) failed")
		spec.Names = []string{"a", "b"}
		spec.Messages = []string{"first", "second"}
		check := NewCheckResult(spec)

		assert.Equal(t, healthv1.SeverityWarning, check.Severity)
		assert.Equal(t, "2 Example(s) failed: first; second", check.Message)
		assert.Equal(t, 2, check.AffectedCount)
		assert.Equal(t, "example.io/v1", check.AffectedResources.APIVersion)
		assert.Equal(t, "Example", check.AffectedResources.Kind)
		assert.Equal(t, map[string]healthv1.AffectedResourceDetail{"a": {}, "b": {}}, check.AffectedResources.Names)
	})

	t.Run("summary is the message when there are no messages", func(t *testing.T) {
		spec := newSpec(healthv1.SeverityInfo, "1 Example(s) are cordoned")
		spec.Names = []string{"a"}
		check := NewCheckResult(spec)

		assert.Equal(t, "1 Example(s) are cordoned", check.Message)
	})

	t.Run("no names gives an empty set", func(t *testing.T) {
		check := NewCheckResult(newSpec(healthv1.SeverityInfo, "none"))

		assert.Equal(t, 0, check.AffectedCount)
		assert.NotNil(t, check.AffectedResources.Names)
		assert.Empty(t, check.AffectedResources.Names)
	})
}

func TestResourceGroupsAdd(t *testing.T) {
	groups := ResourceGroups{}
	groups.Add("reasonA", "a", "same message")
	groups.Add("reasonA", "b", "same message")
	groups.Add("reasonA", "c", "")
	groups.Add("reasonB", "d", "other message")

	assert.Len(t, groups, 2)
	assert.Equal(t, []string{"a", "b", "c"}, groups["reasonA"].Names)
	assert.Equal(t, map[string]struct{}{"same message": {}}, groups["reasonA"].Messages)
	assert.Equal(t, []string{"d"}, groups["reasonB"].Names)
	assert.Equal(t, map[string]struct{}{"other message": {}}, groups["reasonB"].Messages)
}

func TestSortedMessages(t *testing.T) {
	messages := SortedMessages(map[string]struct{}{"b": {}, "c": {}, "a": {}})

	assert.Equal(t, []string{"a", "b", "c"}, messages)
	assert.Empty(t, SortedMessages(nil))
}
