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

package validators

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytcluster"
)

//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurusdatanodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurusdatanodes,verbs=create;update;delete,versions=v1,name=vytsaurusdatanodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurusexecnodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurusexecnodes,verbs=create;update;delete,versions=v1,name=vytsaurusexecnodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurustabletnodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurustabletnodes,verbs=create;update;delete,versions=v1,name=vytsaurustabletnodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurushttpproxies,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurushttpproxies,verbs=create;update;delete,versions=v1,name=vytsaurushttpproxies.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurusrpcproxies,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurusrpcproxies,verbs=create;update;delete,versions=v1,name=vytsaurusrpcproxies.kb.io,admissionReviewVersions=v1

type ytsaurusComponent interface {
	client.Object
	ytcluster.ComponentObject
	comparable
}

func validateYtsaurusComponent[T ytsaurusComponent](ctx context.Context, r *customValidator[T], obj, oldObj T, nameField string) (field.ErrorList, error) {
	var ytsaurusKey client.ObjectKey
	var ytsaurus ytv1.Ytsaurus

	ytsaurusNamePath := field.NewPath("spec", "ytsaurus", "name")
	namePath := ytsaurusNamePath
	if nameField != "" {
		namePath = field.NewPath("spec", nameField)
	}

	if obj != r.nilObject {
		ytsaurusKey = client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetYtsaurusName()}
		if ytsaurusKey.Name == "" {
			return field.ErrorList{field.Required(ytsaurusNamePath, "Ytsaurus name must be specified")}, nil
		}
		if oldObj != r.nilObject {
			if oldObj.GetYtsaurusName() != ytsaurusKey.Name {
				return field.ErrorList{field.Forbidden(ytsaurusNamePath, "Ytsaurus reference is immutable")}, nil
			}
			if oldObj.GetComponentSpec().GetComponent().Name != obj.GetComponentSpec().GetComponent().Name {
				return field.ErrorList{field.Forbidden(namePath, "Component name is immutable")}, nil
			}
		}
	} else if oldObj != r.nilObject {
		ytsaurusKey = client.ObjectKey{Namespace: oldObj.GetNamespace(), Name: oldObj.GetYtsaurusName()}
	} else {
		return nil, nil
	}

	// Admission must observe recent writes, including resources not yet in the cache.
	err := r.APIReader.Get(ctx, ytsaurusKey, &ytsaurus)
	if apierrors.IsNotFound(err) {
		return nil, nil // Allow changing components before cluster.
	}
	if err != nil {
		return nil, err
	}

	cluster := ytcluster.NewCluster(&ytsaurus)
	if err = cluster.FetchComponentObjects(ctx, r.APIReader); err != nil {
		err = fmt.Errorf("failed to fetch cluster components: %w", err)
	} else if oldObj != r.nilObject {
		err = cluster.DeleteComponentObject(oldObj)
	}
	if err != nil {
		return nil, err
	}

	if obj != r.nilObject {
		component := obj.GetComponentSpec().GetComponent()
		if _, exists := cluster.Objects[component]; exists {
			return field.ErrorList{field.Duplicate(namePath, component.Name)}, nil
		}
		if _, exists := cluster.Components[component]; exists {
			return field.ErrorList{field.Duplicate(namePath, component.Name)}, nil
		}
		if err := cluster.AddComponentObject(obj); err != nil {
			return nil, fmt.Errorf("failed to add cluster component: %w", err)
		}
	}

	if err := cluster.LinkComponents(); err != nil {
		return nil, fmt.Errorf("failed to link cluster components: %w", err)
	}

	validator := &ytsaurusValidator{
		customValidator: customValidator[*ytv1.Ytsaurus]{
			Client:    r.Client,
			APIReader: r.APIReader,
			Log:       r.Log,
		},
		componentObject: obj,
	}
	return validator.validateYtsaurus(ctx, cluster, cluster.Ytsaurus), nil
}

func newYtsaurusComponentValidator[T ytsaurusComponent](object T, nameField string) *customValidator[T] {
	r := &customValidator[T]{Object: object}
	r.Validate = func(ctx context.Context, obj, oldObj T) (admission.Warnings, error) {
		allErrors, err := validateYtsaurusComponent(ctx, r, obj, oldObj, nameField)
		// Always allow deleting components - all errors are warnings.
		if obj == r.nilObject {
			warnings := make(admission.Warnings, 0, len(allErrors)+1)
			for _, err := range allErrors {
				warnings = append(warnings, err.Error())
			}
			if err != nil {
				warnings = append(warnings, err.Error())
			}
			return warnings, nil
		}
		if err != nil {
			return nil, err
		}
		if len(allErrors) != 0 {
			gvk, err := r.Client.GroupVersionKindFor(obj)
			if err != nil {
				return nil, err
			}
			return nil, apierrors.NewInvalid(gvk.GroupKind(), obj.GetName(), allErrors)
		}
		return nil, nil
	}
	return r
}

func NewYtsaurusDataNodesValidator() *customValidator[*ytv1.YtsaurusDataNodes] {
	return newYtsaurusComponentValidator(&ytv1.YtsaurusDataNodes{}, "name")
}

func NewYtsaurusExecNodesValidator() *customValidator[*ytv1.YtsaurusExecNodes] {
	return newYtsaurusComponentValidator(&ytv1.YtsaurusExecNodes{}, "name")
}

func NewYtsaurusTabletNodesValidator() *customValidator[*ytv1.YtsaurusTabletNodes] {
	return newYtsaurusComponentValidator(&ytv1.YtsaurusTabletNodes{}, "name")
}

func NewYtsaurusHTTPProxiesValidator() *customValidator[*ytv1.YtsaurusHTTPProxies] {
	return newYtsaurusComponentValidator(&ytv1.YtsaurusHTTPProxies{}, "role")
}

func NewYtsaurusRPCProxiesValidator() *customValidator[*ytv1.YtsaurusRPCProxies] {
	return newYtsaurusComponentValidator(&ytv1.YtsaurusRPCProxies{}, "role")
}
