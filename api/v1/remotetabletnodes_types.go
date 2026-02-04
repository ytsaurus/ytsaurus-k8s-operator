/*
Copyright 2023.

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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RemoteTabletNodesSpec defines the desired state of RemoteTabletNodes
type RemoteTabletNodesSpec struct {
	RemoteClusterSpec *corev1.LocalObjectReference `json:"remoteClusterSpec"`
	CommonSpec        `json:",inline"`
	TabletNodesSpec   `json:",inline"`
}

// RemoteTabletNodesStatus defines the observed state of RemoteTabletNodes
type RemoteTabletNodesStatus struct {
	CommonRemoteNodeStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
//+kubebuilder:printcolumn:name="ReleaseStatus",type="string",JSONPath=".status.releaseStatus",description="Release status"
//+kubebuilder:resource:categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status

// RemoteTabletNodes is the Schema for the remotetabletnodes API
type RemoteTabletNodes struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteTabletNodesSpec   `json:"spec,omitempty"`
	Status RemoteTabletNodesStatus `json:"status,omitempty"`
}

func (r *RemoteTabletNodes) GetStatusObservedGeneration() int64 {
	return r.Status.ObservedGeneration
}

func (r *RemoteTabletNodes) SetStatusObservedGeneration(generation int64) {
	r.Status.ObservedGeneration = generation
}

func (r *RemoteTabletNodes) GetStatusConditions() []metav1.Condition {
	return r.Status.Conditions
}

func (r *RemoteTabletNodes) SetStatusConditions(conditions []metav1.Condition) {
	r.Status.Conditions = conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"

// RemoteTabletNodesList contains a list of RemoteTabletNodes
type RemoteTabletNodesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteTabletNodes `json:"items"`
}
