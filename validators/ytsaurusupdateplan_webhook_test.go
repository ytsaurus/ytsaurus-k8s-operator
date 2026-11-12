package validators

import (
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func testUpdatePlan() *ytv1.YtsaurusUpdatePlan {
	return &ytv1.YtsaurusUpdatePlan{
		ObjectMeta: metav1.ObjectMeta{Name: "plan", Namespace: "test"},
		Spec: ytv1.YtsaurusUpdatePlanSpec{
			Ytsaurus:   ytv1.YtsaurusReference{Name: "cluster"},
			UpdatePlan: ytv1.UpdatePlan{{Class: ytv1.ComponentClassStateless}},
		},
	}
}

func TestUpdatePlanValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*ytv1.YtsaurusUpdatePlan)
		message string
	}{
		{"valid before cluster exists", func(*ytv1.YtsaurusUpdatePlan) {}, ""},
		{"empty plan", func(p *ytv1.YtsaurusUpdatePlan) { p.Spec.UpdatePlan = nil }, ""},
		{"garbage collector", func(p *ytv1.YtsaurusUpdatePlan) {
			p.Spec.UpdatePlan = ytv1.UpdatePlan{{Component: ytv1.Component{Type: ytv1.GarbageCollectorType}}}
		}, ""},
		{"garbage collector rolling update", func(p *ytv1.YtsaurusUpdatePlan) {
			p.Spec.UpdatePlan = ytv1.UpdatePlan{{
				Component: ytv1.Component{Type: ytv1.GarbageCollectorType},
				Strategy:  &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}},
			}}
		}, "GarbageCollector supports only BulkUpdate mode"},
		{"garbage collector on-delete update", func(p *ytv1.YtsaurusUpdatePlan) {
			p.Spec.UpdatePlan = ytv1.UpdatePlan{{
				Component: ytv1.Component{Type: ytv1.GarbageCollectorType},
				Strategy:  &ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}},
			}}
		}, "GarbageCollector supports only BulkUpdate mode"},
		{"missing parent", func(p *ytv1.YtsaurusUpdatePlan) { p.Spec.Ytsaurus.Name = "" }, "spec.ytsaurus.name"},
		{"invalid parent", func(p *ytv1.YtsaurusUpdatePlan) { p.Spec.Ytsaurus.Name = "invalid/name" }, "spec.ytsaurus.name"},
		{"empty selector", func(p *ytv1.YtsaurusUpdatePlan) { p.Spec.UpdatePlan = ytv1.UpdatePlan{{}} }, "Either component or class"},
		{"mixed selector", func(p *ytv1.YtsaurusUpdatePlan) { p.Spec.UpdatePlan[0].Component.Type = ytv1.MasterType }, "Only one of component or class"},
		{"unknown type", func(p *ytv1.YtsaurusUpdatePlan) {
			p.Spec.UpdatePlan = ytv1.UpdatePlan{{Component: ytv1.Component{Type: "Unknown"}}}
		}, "Unknown component type"},
		{"class strategy", func(p *ytv1.YtsaurusUpdatePlan) {
			p.Spec.UpdatePlan[0].Strategy = &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}
		}, "strategy is supported only for specific components"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := testUpdatePlan()
			tc.change(plan)
			v := componentTestClient(NewYtsaurusUpdatePlanValidator(), clusterTestClient(t))
			_, err := v.ValidateCreate(t.Context(), plan)
			if tc.message == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.message)
			}
		})
	}
}

func TestUpdatePlanOwnershipValidation(t *testing.T) {
	plan := testUpdatePlan()
	v := componentTestClient(NewYtsaurusUpdatePlanValidator(), clusterTestClient(t, plan))
	_, err := v.ValidateUpdate(t.Context(), plan, plan.DeepCopy())
	require.NoError(t, err)
	other := plan.DeepCopy()
	other.Name = "duplicate"
	_, err = v.ValidateCreate(t.Context(), other)
	require.ErrorContains(t, err, "Duplicate value")
	other.Name = plan.Name
	other.Spec.Ytsaurus.Name = "other"
	_, err = v.ValidateUpdate(t.Context(), plan, other)
	require.ErrorContains(t, err, "Ytsaurus reference is immutable")
	other.Name = "other"
	_, err = v.ValidateCreate(t.Context(), other)
	require.NoError(t, err)
	other = plan.DeepCopy()
	other.Namespace = "other"
	_, err = v.ValidateCreate(t.Context(), other)
	require.NoError(t, err)
}

func TestUpdatePlanInlineConflictValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		plan ytv1.UpdatePlan
	}{
		{"omitted", nil},
		{"empty", ytv1.UpdatePlan{}},
		{"everything", ytv1.UpdatePlan{{Class: ytv1.ComponentClassEverything}}},
		{"nothing", ytv1.UpdatePlan{{Class: ytv1.ComponentClassNothing}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := testUpdatePlan()
			parent, _, _ := componentTestObjects("YtsaurusDataNodes")
			parent.Spec.UpdatePlan = tc.plan
			c := clusterTestClient(t, parent)
			v := componentTestClient(NewYtsaurusUpdatePlanValidator(), c)
			_, createErr := v.ValidateCreate(t.Context(), plan)
			require.NoError(t, c.Create(t.Context(), plan))
			_, updateErr := v.ValidateUpdate(t.Context(), plan, plan.DeepCopy())
			for _, err := range []error{createErr, updateErr} {
				if len(tc.plan) == 0 {
					require.NoError(t, err)
				} else {
					require.True(t, apierrors.IsInvalid(err), "expected an admission validation error, got %v", err)
					require.ErrorContains(t, err, "already has an inline update plan")
				}
			}
		})
	}
}

func TestUpdatePlanValidatesWholeCluster(t *testing.T) {
	for _, invalidComponent := range []bool{false, true} {
		name := "parent"
		if invalidComponent {
			name = "separate component"
		}
		t.Run(name, func(t *testing.T) {
			parent, component, instance := componentTestObjects("YtsaurusDataNodes")
			if invalidComponent {
				instance.VolumeMounts[0].MountPath += "/"
			} else {
				parent.Spec.PrimaryMasters.Locations = nil
			}
			plan := testUpdatePlan()
			c := clusterTestClient(t, parent, component)
			v := componentTestClient(NewYtsaurusUpdatePlanValidator(), c)
			_, err := v.ValidateCreate(t.Context(), plan)
			require.True(t, apierrors.IsInvalid(err), "expected a cluster validation error, got %v", err)
			message := "spec.primaryMasters.locations"
			if invalidComponent {
				message = "spec.volumeMounts[0]"
			}
			require.ErrorContains(t, err, message)
			require.NoError(t, c.Create(t.Context(), plan))
			_, err = v.ValidateUpdate(t.Context(), plan, plan.DeepCopy())
			require.True(t, apierrors.IsInvalid(err), "expected a cluster validation error, got %v", err)
			require.ErrorContains(t, err, message)
		})
	}
}

func TestUpdatePlanValidatesIncomingSpec(t *testing.T) {
	parent, _, _ := componentTestObjects("YtsaurusDataNodes")
	plan := testUpdatePlan()
	stored := plan.DeepCopy()
	stored.Spec.UpdatePlan = ytv1.UpdatePlan{{}}
	v := componentTestClient(NewYtsaurusUpdatePlanValidator(), clusterTestClient(t, parent, stored))
	_, err := v.ValidateUpdate(t.Context(), stored, plan)
	require.NoError(t, err)
}

func TestUpdatePlanValidatesSeparateDataNodeGroups(t *testing.T) {
	parent, component, _ := componentTestObjects("YtsaurusDataNodes")
	inline := component.(*ytv1.YtsaurusDataNodes).Spec.DataNodesSpec.DeepCopy()
	inline.Name = "inline"
	parent.Spec.DataNodes = []ytv1.DataNodesSpec{*inline}
	plan := testUpdatePlan()
	plan.Spec.UpdatePlan = ytv1.UpdatePlan{{
		Component: ytv1.Component{Type: ytv1.DataNodeType},
		Strategy:  &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}},
	}}
	v := componentTestClient(NewYtsaurusUpdatePlanValidator(), clusterTestClient(t, parent, component))
	_, err := v.ValidateCreate(t.Context(), plan)
	require.ErrorContains(t, err, "rolling update for several data node groups requires concurrency limit")
	plan.Spec.UpdatePlan[0].Concurrency = ptr.To(int32(1))
	_, err = v.ValidateCreate(t.Context(), plan)
	require.NoError(t, err)

	// Creating another data node group must validate the existing separate plan too.
	plan.Spec.UpdatePlan[0].Concurrency = nil
	parentOnly := clusterTestClient(t, parent, plan)
	componentValidator := componentTestClient(NewYtsaurusDataNodesValidator(), parentOnly)
	_, err = componentValidator.ValidateCreate(t.Context(), component)
	require.ErrorContains(t, err, "rolling update for several data node groups requires concurrency limit")
}
