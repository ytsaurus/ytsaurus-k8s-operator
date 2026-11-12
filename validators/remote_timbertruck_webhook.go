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

	"k8s.io/apimachinery/pkg/util/validation/field"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-remotedatanodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=remotedatanodes,verbs=create;update,versions=v1,name=vremotedatanodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-remoteexecnodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=remoteexecnodes,verbs=create;update,versions=v1,name=vremoteexecnodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-remotetabletnodes,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=remotetabletnodes,verbs=create;update,versions=v1,name=vremotetabletnodes.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-offshoredatagateways,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=offshoredatagateways,verbs=create;update,versions=v1,name=voffshoredatagateways.kb.io,admissionReviewVersions=v1

func validateRemoteTimbertruck(commonSpec *ytv1.CommonSpec, instanceSpec *ytv1.InstanceSpec) Results {
	var results Results
	specPath := field.NewPath("spec")
	if commonSpec.Timbertruck != nil {
		results.Error(field.Forbidden(specPath.Child("timbertruck"), "timbertruck delivery is not supported for remote resources"))
	}
	for i, logger := range instanceSpec.StructuredLoggers {
		if logger.EnableDelivery != nil {
			results.Error(field.Forbidden(
				specPath.Child("structuredLoggers").Index(i).Child("enableDelivery"),
				"timbertruck delivery is not supported for remote resources",
			))
		}
	}
	results.Append(validateStructuredLoggers(instanceSpec.StructuredLoggers, specPath))
	return results
}

type remoteDataNodesValidator struct {
	customValidator[*ytv1.RemoteDataNodes]
}

func NewRemoteDataNodesValidator() *remoteDataNodesValidator {
	r := &remoteDataNodesValidator{}
	r.Object = &ytv1.RemoteDataNodes{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.RemoteDataNodes) Results {
		if obj == nil {
			return Results{}
		}
		return validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec)
	}
	return r
}

type remoteExecNodesValidator struct {
	customValidator[*ytv1.RemoteExecNodes]
}

func NewRemoteExecNodesValidator() *remoteExecNodesValidator {
	r := &remoteExecNodesValidator{}
	r.Object = &ytv1.RemoteExecNodes{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.RemoteExecNodes) Results {
		if obj == nil {
			return Results{}
		}
		return validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec)
	}
	return r
}

type remoteTabletNodesValidator struct {
	customValidator[*ytv1.RemoteTabletNodes]
}

func NewRemoteTabletNodesValidator() *remoteTabletNodesValidator {
	r := &remoteTabletNodesValidator{}
	r.Object = &ytv1.RemoteTabletNodes{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.RemoteTabletNodes) Results {
		if obj == nil {
			return Results{}
		}
		return validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec)
	}
	return r
}

type offshoreDataGatewaysValidator struct {
	customValidator[*ytv1.OffshoreDataGateways]
}

func NewOffshoreDataGatewaysValidator() *offshoreDataGatewaysValidator {
	r := &offshoreDataGatewaysValidator{}
	r.Object = &ytv1.OffshoreDataGateways{}
	r.Validate = func(_ context.Context, obj, _ *ytv1.OffshoreDataGateways) Results {
		if obj == nil {
			return Results{}
		}
		return validateRemoteTimbertruck(&obj.Spec.CommonSpec, &obj.Spec.InstanceSpec)
	}
	return r
}
