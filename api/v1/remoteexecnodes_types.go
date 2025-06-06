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

// RemoteExecNodesSpec defines the desired state of RemoteExecNodes
type RemoteExecNodesSpec struct {
	RemoteClusterSpec *corev1.LocalObjectReference `json:"remoteClusterSpec"`
	CommonSpec        `json:",inline"`
	ExecNodesSpec     `json:",inline"`
}

// RemoteExecNodesStatus defines the observed state of RemoteExecNodes
type RemoteExecNodesStatus struct {
	CommonRemoteNodeStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:printcolumn:name="ReleaseStatus",type="string",JSONPath=".status.releaseStatus",description="Release status"
//+kubebuilder:resource:categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status

// RemoteExecNodes is the Schema for the remoteexecnodes API
type RemoteExecNodes struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteExecNodesSpec   `json:"spec,omitempty"`
	Status RemoteExecNodesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RemoteExecNodesList contains a list of RemoteExecNodes
type RemoteExecNodesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteExecNodes `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteExecNodes{}, &RemoteExecNodesList{})
}
