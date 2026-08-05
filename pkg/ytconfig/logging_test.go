package ytconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestCreateStructuredLoggingRule(t *testing.T) {
	for _, testCase := range []struct {
		name              string
		spec              ytv1.StructuredLoggerSpec
		includeCategories []string
		excludeCategories []string
	}{
		{
			name:              "category",
			spec:              ytv1.StructuredLoggerSpec{Category: "Access"},
			includeCategories: []string{"Access"},
		},
		{
			// The scheduler event log spans both of these.
			name: "include filter replaces category",
			spec: ytv1.StructuredLoggerSpec{CategoriesFilter: &ytv1.CategoriesFilter{
				Type:   ytv1.CategoriesFilterTypeInclude,
				Values: []string{"SchedulerStructuredLog", "SchedulerEventLog"},
			}},
			includeCategories: []string{"SchedulerStructuredLog", "SchedulerEventLog"},
		},
		{
			name: "exclude filter",
			spec: ytv1.StructuredLoggerSpec{CategoriesFilter: &ytv1.CategoriesFilter{
				Type:   ytv1.CategoriesFilterTypeExclude,
				Values: []string{"Barrier"},
			}},
			excludeCategories: []string{"Barrier"},
		},
		{
			// An empty category must not turn into an absent include list, which matches everything.
			name:              "empty category",
			spec:              ytv1.StructuredLoggerSpec{},
			includeCategories: []string{""},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			rule := createStructuredLoggingRule(testCase.spec)
			require.Equal(t, testCase.includeCategories, rule.IncludeCategories)
			require.Equal(t, testCase.excludeCategories, rule.ExcludeCategories)
		})
	}
}
