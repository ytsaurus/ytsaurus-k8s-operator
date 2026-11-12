/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// YtsaurusUpdatePlanSpec defines which components may update and their strategies.
type YtsaurusUpdatePlanSpec struct {
	// Cluster to update in the same namespace. Immutable after creation.
	Ytsaurus YtsaurusReference `json:"ytsaurus"`
	// Update plan defines components which are allowed to update.
	// Can contain either single "class" item or several "component" items.
	// When empty: update nothing
	UpdatePlan UpdatePlan `json:"updatePlan,omitempty"`
	// Delete this resource after all selected updates finish successfully.
	// A plan waiting for updates or blocked from executing is retained.
	OneShot bool `json:"oneShot,omitempty"`
}

//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurusupdateplan,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurusupdateplan/status,verbs=get;update;patch

// YtsaurusUpdatePlan controls updates for one Ytsaurus cluster in the same namespace.
// +kubebuilder:object:root=true
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
// +kubebuilder:resource:path=ytsaurusupdateplan,shortName=ytup,categories=ytsaurus-all;yt-all
// +kubebuilder:printcolumn:name="Ytsaurus",type="string",JSONPath=".spec.ytsaurus.name",description="Ytsaurus cluster"
// +kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="Update plan state"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message",description="Status message"

// +kubebuilder:subresource:status
type YtsaurusUpdatePlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	YtsaurusComponentBase `json:",inline"`

	Spec YtsaurusUpdatePlanSpec `json:"spec"`
}

func (y *YtsaurusUpdatePlan) GetYtsaurusName() string {
	return y.Spec.Ytsaurus.Name
}

func (y *YtsaurusUpdatePlan) GetComponentSpec() ComponentSpec {
	return &y.Spec
}

func (n *YtsaurusUpdatePlanSpec) GetComponent() Component {
	return Component{UpdatePlanType, ""}
}

func (n *YtsaurusUpdatePlanSpec) GetInstanceCount() int {
	return 0
}

// +kubebuilder:object:root=true
// +kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
type YtsaurusUpdatePlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YtsaurusUpdatePlan `json:"items"`
}
