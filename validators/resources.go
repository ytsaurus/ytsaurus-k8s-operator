package validators

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

func hasGuaranteedResources(resources corev1.ResourceRequirements) bool {
	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		limit := resources.Limits[name]
		request, ok := resources.Requests[name]
		if !ok {
			// Kubernetes defaults an omitted request to its limit.
			request = limit
		}
		if limit.Sign() <= 0 || !request.Equal(limit) {
			return false
		}
	}
	return true
}

func validateGuaranteedResources(resources corev1.ResourceRequirements, path *field.Path) Results {
	var results Results
	if !hasGuaranteedResources(resources) {
		results.Warning(field.Invalid(path, resources, "CPU and memory requests should equal positive limits to preserve Guaranteed pod QoS"))
	}
	return results
}

func (r *baseValidator) validateContainers(containers []string, mainResources corev1.ResourceRequirements, initContainer bool, path *field.Path) Results {
	var results Results
	guaranteed := hasGuaranteedResources(mainResources)
	names := make(map[string]bool)
	for i, containerSpec := range containers {
		container := corev1.Container{}
		if err := yaml.UnmarshalStrict([]byte(containerSpec), &container); err != nil {
			results.Error(field.Invalid(path.Index(i), containerSpec, err.Error()))
			continue
		}
		if names[container.Name] {
			results.Error(field.Duplicate(path.Index(i).Child("name"), container.Name))
		}
		names[container.Name] = true
		if guaranteed {
			resourcesPath := path.Index(i).Child("resources")
			results.Append(validateGuaranteedResources(container.Resources, resourcesPath))
			cpu := container.Resources.Limits.Cpu()
			if initContainer && cpu.Sign() > 0 && cpu.MilliValue()%1000 == 0 {
				results.Warning(field.Invalid(resourcesPath.Child("limits", "cpu"), cpu.String(),
					"init container CPU should be fractional to avoid leaking cores with the static CPU manager (kubernetes/kubernetes#112228)"))
			}
		}
	}
	return results
}

func (r *baseValidator) validateExecNodeResources(spec *ytv1.ExecNodesSpec, path *field.Path) Results {
	var results Results
	results.Append(r.validateContainers(spec.InitContainers, spec.Resources, true, path.Child("initContainers")))
	results.Append(r.validateContainers(spec.Sidecars, spec.Resources, false, path.Child("sidecars")))
	if !hasGuaranteedResources(spec.Resources) {
		return results
	}

	cri := ytconfig.NewCRIConfigGenerator(spec)
	jobPath := path.Child("jobResources")
	if cri.Isolated {
		if cri.Service == ytv1.CRIServiceNone {
			return results
		}
		if spec.JobResources == nil {
			results.Warning(field.Required(jobPath, "isolated jobs should have Guaranteed resources to preserve Guaranteed pod QoS"))
		} else {
			results.Append(validateGuaranteedResources(*spec.JobResources, jobPath))
		}
		return results
	}

	if spec.JobResources != nil {
		// Without isolation, job resources are added to the main container before
		// Kubernetes defaults requests. Validate the resulting resources.
		resources := spec.Resources.DeepCopy()
		resources.Requests = addResourceLists(resources.Requests, spec.JobResources.Requests)
		resources.Limits = addResourceLists(resources.Limits, spec.JobResources.Limits)
		if !hasGuaranteedResources(*resources) {
			results.Warning(field.Invalid(jobPath, spec.JobResources, "job resources should preserve Guaranteed pod QoS"))
		}
	}
	return results
}

func addResourceLists(resources, extra corev1.ResourceList) corev1.ResourceList {
	if resources == nil {
		resources = corev1.ResourceList{}
	}
	for name, quantity := range extra {
		value := resources[name]
		value.Add(quantity)
		resources[name] = value
	}
	return resources
}

func validateTimbertruckQoS(spec ytv1.InstanceSpec, timbertruck *ytv1.TimbertruckSpec, path *field.Path) Results {
	var results Results
	if !hasGuaranteedResources(spec.Resources) {
		return results
	}
	const detail = "timbertruck sidecar has no resource limits and cannot preserve Guaranteed pod QoS"
	if timbertruck != nil {
		results.Warning(field.Forbidden(path.Child("timbertruck"), detail))
	}
	for i, logger := range spec.StructuredLoggers {
		if logger.EnableDelivery != nil && *logger.EnableDelivery {
			results.Warning(field.Forbidden(path.Child("structuredLoggers").Index(i).Child("enableDelivery"), detail))
		}
	}
	return results
}
