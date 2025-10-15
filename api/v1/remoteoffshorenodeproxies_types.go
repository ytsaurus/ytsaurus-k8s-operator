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

// RemoteOffshoreNodeProxiesSpec defines the desired state of RemoteOffshoreNodeProxies
type RemoteOffshoreNodeProxiesSpec struct {
	RemoteClusterSpec       *corev1.LocalObjectReference `json:"remoteClusterSpec"`
	CommonSpec              `json:",inline"`
	OffshoreNodeProxiesSpec `json:",inline"`
}

// RemoteOffshoreNodeProxiesStatus defines the observed state of RemoteOffshoreNodeProxies
type RemoteOffshoreNodeProxiesStatus struct {
	CommonRemoteNodeStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:printcolumn:name="ReleaseStatus",type="string",JSONPath=".status.releaseStatus",description="Release status"
//+kubebuilder:resource:categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status

// RemoteOffshoreNodeProxies is the Schema for the RemoteOffshoreNodeProxies API
type RemoteOffshoreNodeProxies struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteOffshoreNodeProxiesSpec   `json:"spec,omitempty"`
	Status RemoteOffshoreNodeProxiesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RemoteOffshoreNodeProxiesList contains a list of RemoteOffshoreNodeProxies
type RemoteOffshoreNodeProxiesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteOffshoreNodeProxies `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteOffshoreNodeProxies{}, &RemoteOffshoreNodeProxiesList{})
}
