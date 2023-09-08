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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ChytReleaseStatus string

const (
	ChytReleaseStatusCreatingUserSecret     ChytReleaseStatus = "CreatingUserSecret"
	ChytReleaseStatusCreatingUser           ChytReleaseStatus = "CreatingUser"
	ChytReleaseStatusUploadingIntoCypress   ChytReleaseStatus = "UploadingIntoCypress"
	ChytReleaseStatusCreatingChPublicClique ChytReleaseStatus = "CreatingChPublicClique"
	ChytReleaseStatusFinished               ChytReleaseStatus = "Finished"
)

// ChytSpec defines the desired state of Chyt
type ChytSpec struct {
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	Ytsaurus *corev1.LocalObjectReference `json:"ytsaurus,omitempty"`
	Image    string                       `json:"image,omitempty"`
	//+kubebuilder:default:=false
	MakeDefault bool `json:"makeDefault"`
}

// ChytStatus defines the observed state of Chyt
type ChytStatus struct {
	Conditions    []metav1.Condition `json:"conditions,omitempty"`
	ReleaseStatus ChytReleaseStatus  `json:"releaseStatus,omitempty"`
}

//+kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="ReleaseStatus",type="string",JSONPath=".status.releaseStatus",description="Status of release"
//+kubebuilder:subresource:status

// Chyt is the Schema for the chyts API
type Chyt struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ChytSpec   `json:"spec,omitempty"`
	Status ChytStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ChytList contains a list of Chyt
type ChytList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Chyt `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Chyt{}, &ChytList{})
}
