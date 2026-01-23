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

type SpytReleaseStatus string

const (
	SpytReleaseStatusCreatingUserSecret   SpytReleaseStatus = "CreatingUserSecret"
	SpytReleaseStatusCreatingUser         SpytReleaseStatus = "CreatingUser"
	SpytReleaseStatusUploadingIntoCypress SpytReleaseStatus = "UploadingIntoCypress"
	SpytReleaseStatusFinished             SpytReleaseStatus = "Finished"
)

// SpytSpec defines the desired state of Spyt
type SpytSpec struct {
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	Ytsaurus *corev1.LocalObjectReference `json:"ytsaurus,omitempty"`
	Image    string                       `json:"image,omitempty"`
	// If multiple versions are provided, only the first one is used for query tracker dynamic config.
	SparkVersions []string `json:"sparkVersions,omitempty"`
	// Use only local artifacts for spark distribution setup.
	SparkDistribOffline bool `json:"sparkDistribOffline,omitempty"`
}

// SpytStatus defines the observed state of Spyt
type SpytStatus struct {
	Conditions    []metav1.Condition `json:"conditions,omitempty"`
	ReleaseStatus SpytReleaseStatus  `json:"releaseStatus,omitempty"`
}

//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=spyts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=spyts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=spyts/finalizers,verbs=update

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
//+kubebuilder:printcolumn:name="ReleaseStatus",type="string",JSONPath=".status.releaseStatus",description="Status of release"
//+kubebuilder:resource:categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status

// Spyt is the Schema for the spyts API
type Spyt struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SpytSpec   `json:"spec,omitempty"`
	Status            SpytStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"

// SpytList contains a list of Spyt
type SpytList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Spyt `json:"items"`
}
