/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package validators

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-remotedatanodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=remotedatanodes,verbs=create;update,versions=v1,name=vremotedatanodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-remoteexecnodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=remoteexecnodes,verbs=create;update,versions=v1,name=vremoteexecnodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-remotetabletnodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=remotetabletnodes,verbs=create;update,versions=v1,name=vremotetabletnodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-offshoredatagateways,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=offshoredatagateways,verbs=create;update,versions=v1,name=voffshoredatagateways.kb.io,admissionReviewVersions=v1

func validateRemoteTimbertruck(commonSpec *ytv1.CommonSpec, instanceSpec *ytv1.InstanceSpec) field.ErrorList {
	var allErrors field.ErrorList
	specPath := field.NewPath("spec")
	if commonSpec.Timbertruck != nil {
		allErrors = append(allErrors, field.Forbidden(specPath.Child("timbertruck"), "timbertruck delivery is not supported for remote resources"))
	}
	for i, logger := range instanceSpec.StructuredLoggers {
		if logger.EnableDelivery != nil {
			allErrors = append(allErrors, field.Forbidden(
				specPath.Child("structuredLoggers").Index(i).Child("enableDelivery"),
				"timbertruck delivery is not supported for remote resources",
			))
		}
	}
	return allErrors
}

func remoteValidationError(gvk schema.GroupVersionKind, name string, errors field.ErrorList) error {
	if len(errors) == 0 {
		return nil
	}
	return apierrors.NewInvalid(gvk.GroupKind(), name, errors)
}

type remoteDataNodesValidator struct {
	customValidator[*ytv1.RemoteDataNodes]
}

func NewRemoteDataNodesValidator() *remoteDataNodesValidator {
	r := &remoteDataNodesValidator{}
	r.Object = &ytv1.RemoteDataNodes{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.RemoteDataNodes) (admission.Warnings, error) {
		return nil, remoteValidationError(ytv1.GroupVersion.WithKind("RemoteDataNodes"), obj.Name, validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec))
	}
	return r
}

type remoteExecNodesValidator struct {
	customValidator[*ytv1.RemoteExecNodes]
}

func NewRemoteExecNodesValidator() *remoteExecNodesValidator {
	r := &remoteExecNodesValidator{}
	r.Object = &ytv1.RemoteExecNodes{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.RemoteExecNodes) (admission.Warnings, error) {
		return nil, remoteValidationError(ytv1.GroupVersion.WithKind("RemoteExecNodes"), obj.Name, validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec))
	}
	return r
}

type remoteTabletNodesValidator struct {
	customValidator[*ytv1.RemoteTabletNodes]
}

func NewRemoteTabletNodesValidator() *remoteTabletNodesValidator {
	r := &remoteTabletNodesValidator{}
	r.Object = &ytv1.RemoteTabletNodes{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.RemoteTabletNodes) (admission.Warnings, error) {
		return nil, remoteValidationError(ytv1.GroupVersion.WithKind("RemoteTabletNodes"), obj.Name, validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec))
	}
	return r
}

type offshoreDataGatewaysValidator struct {
	customValidator[*ytv1.OffshoreDataGateways]
}

func NewOffshoreDataGatewaysValidator() *offshoreDataGatewaysValidator {
	r := &offshoreDataGatewaysValidator{}
	r.Object = &ytv1.OffshoreDataGateways{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.OffshoreDataGateways) (admission.Warnings, error) {
		return nil, remoteValidationError(ytv1.GroupVersion.WithKind("OffshoreDataGateways"), obj.Name, validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec))
	}
	return r
}
