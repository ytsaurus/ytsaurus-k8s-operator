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

package validators

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-chyt,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=chyts,verbs=create;update,versions=v1,name=vchyt.kb.io,admissionReviewVersions=v1

type chytValidator struct {
	customValidator[*ytv1.Chyt]
}

func NewChytValidator() *chytValidator {
	r := &chytValidator{}
	r.Object = &ytv1.Chyt{}
	r.Validate = r.evaluateChytValidation
	return r
}

func (r *chytValidator) evaluateChytValidation(ctx context.Context, newChyt, oldChyt *ytv1.Chyt) (admission.Warnings, error) {
	return nil, nil
}
