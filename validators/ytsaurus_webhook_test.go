package validators

import (
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestYtsaurusValidatorReadsUncachedComponents(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "accepts valid uncached components"
		if !valid {
			name = "rejects invalid uncached components"
		}
		t.Run(name, func(t *testing.T) {
			parent, obj, _ := componentTestObjects("YtsaurusHTTPProxies")
			parent.Spec.HTTPProxies = nil
			proxy := obj.(*ytv1.YtsaurusHTTPProxies)
			proxy.Spec.Role = "default"
			if !valid {
				proxy.Spec.PodLabels = map[string]string{"invalid key": "value"}
			}
			validator := NewYtsaurusValidator()
			validator.Client = clusterTestClient(t)
			validator.APIReader = clusterTestClient(t, proxy)

			_, createErr := validator.ValidateCreate(t.Context(), parent)
			_, updateErr := validator.ValidateUpdate(t.Context(), parent.DeepCopy(), parent)
			for _, err := range []error{createErr, updateErr} {
				if valid {
					require.NoError(t, err)
				} else {
					require.True(t, apierrors.IsInvalid(err), "%v", err)
					require.ErrorContains(t, err, "spec.podLabels")
				}
			}
		})
	}
}

func TestYtsaurusComponentValidatorReadsUncachedParent(t *testing.T) {
	parent, obj, _ := componentTestObjects("YtsaurusHTTPProxies")
	proxy := obj.(*ytv1.YtsaurusHTTPProxies)
	stored := proxy.DeepCopy()
	proxy.Spec.PodLabels = map[string]string{"invalid key": "value"}
	validator := NewYtsaurusHTTPProxiesValidator()
	validator.Client = clusterTestClient(t)
	validator.APIReader = clusterTestClient(t, parent)
	_, createErr := validator.ValidateCreate(t.Context(), proxy)
	validator.APIReader = clusterTestClient(t, parent, stored)
	_, updateErr := validator.ValidateUpdate(t.Context(), stored, proxy)
	for _, err := range []error{createErr, updateErr} {
		require.True(t, apierrors.IsInvalid(err), "%v", err)
		require.ErrorContains(t, err, "spec.podLabels")
	}
}

func TestYtsaurusComponentValidatorRejectsUncachedDuplicate(t *testing.T) {
	parent, obj, _ := componentTestObjects("YtsaurusHTTPProxies")
	proxy := obj.(*ytv1.YtsaurusHTTPProxies)
	otherProxy := proxy.DeepCopy()
	otherProxy.Name = "another-proxy"
	validator := NewYtsaurusHTTPProxiesValidator()
	validator.Client = clusterTestClient(t)
	validator.APIReader = clusterTestClient(t, parent, otherProxy)

	_, err := validator.ValidateCreate(t.Context(), proxy)
	require.True(t, apierrors.IsInvalid(err), "%v", err)
	require.ErrorContains(t, err, "spec.role: Duplicate value")
}

func TestYtsaurusValidatorControllerAgentsRequireSchedulers(t *testing.T) {
	parent, _, _ := componentTestObjects("YtsaurusDataNodes")
	parent.Spec.Schedulers = nil
	parent.Spec.ControllerAgents.InstanceCount = 1
	validator := NewYtsaurusValidator()
	validator.Client = clusterTestClient(t)
	validator.APIReader = validator.Client
	_, createErr := validator.ValidateCreate(t.Context(), parent)
	_, updateErr := validator.ValidateUpdate(t.Context(), parent.DeepCopy(), parent)
	for _, err := range []error{createErr, updateErr} {
		require.True(t, apierrors.IsInvalid(err), "%v", err)
		require.ErrorContains(t, err, "spec.schedulers: Required value: controllerAgents doesn't make sense without schedulers")
	}
}
