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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RemoteYtsaurusSpec defines the desired state of RemoteYtsaurus
type RemoteYtsaurusSpec struct {
	MasterConnectionSpec `json:",inline"`
	MasterCachesSpec     `json:",inline"`
}

// RemoteYtsaurusStatus defines the observed state of RemoteYtsaurus
type RemoteYtsaurusStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
//+kubebuilder:resource:path=remoteytsaurus,categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status

// RemoteYtsaurus is the Schema for the remoteytsauruses API
type RemoteYtsaurus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RemoteYtsaurusSpec   `json:"spec,omitempty"`
	Status RemoteYtsaurusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"

// RemoteYtsaurusList contains a list of RemoteYtsaurus
type RemoteYtsaurusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteYtsaurus `json:"items"`
}

// RemoteYtsaurus doesn't have a reconciller, so we put rbac markers here.
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=remoteytsaurus,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=remoteytsaurus/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=remoteytsaurus/finalizers,verbs=update
