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

// OffshoreDataGatewaysSpec defines the desired state of OffshoreDataGateways
type OffshoreDataGatewaysSpec struct {
	RemoteClusterSpec       *corev1.LocalObjectReference `json:"remoteClusterSpec"`
	CommonSpec              `json:",inline"`
	OffshoreDataGatewaySpec `json:",inline"`
}

// OffshoreDataGatewaysStatus defines the observed state of OffshoreDataGateways
type OffshoreDataGatewaysStatus struct {
	CommonRemoteNodeStatus `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
//+kubebuilder:printcolumn:name="ReleaseStatus",type="string",JSONPath=".status.releaseStatus",description="Release status"
//+kubebuilder:resource:categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status

// OffshoreDataGateways is the Schema for the OffshoreDataGateways API
// Be careful: this component is experimental and is not part of the public
// API yet, so there are no guarantees it will work even when configured
// TODO(pavel-bash): remove this warning when the component is released to open-source
type OffshoreDataGateways struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OffshoreDataGatewaysSpec   `json:"spec,omitempty"`
	Status OffshoreDataGatewaysStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"

// OffshoreDataGatewaysList contains a list of OffshoreDataGateways
type OffshoreDataGatewaysList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OffshoreDataGateways `json:"items"`
}
