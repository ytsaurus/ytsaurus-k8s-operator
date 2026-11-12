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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type YtsaurusHTTPProxiesSpec struct {
	Ytsaurus        YtsaurusReference `json:"ytsaurus"`
	HTTPProxiesSpec `json:",inline"`
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
//+kubebuilder:printcolumn:name="CName",type="string",JSONPath=".spec.role",description="Component name"
//+kubebuilder:printcolumn:name="State",type="string",JSONPath=".status.state",description="State of component"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.message",description="Status message"
//+kubebuilder:resource:categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurushttpproxies,verbs=get;list;watch
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurushttpproxies/status,verbs=get;update;patch

type YtsaurusHTTPProxies struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	YtsaurusComponentBase `json:",inline"`

	Spec YtsaurusHTTPProxiesSpec `json:"spec"`
}

func (y *YtsaurusHTTPProxies) GetYtsaurusName() string {
	return y.Spec.Ytsaurus.Name
}

func (y *YtsaurusHTTPProxies) GetComponentSpec() ComponentSpec {
	return &y.Spec.HTTPProxiesSpec
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"

type YtsaurusHTTPProxiesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []YtsaurusHTTPProxies `json:"items"`
}
