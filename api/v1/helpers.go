package v1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"
)

func FindFirstLocation(locations []LocationSpec, locationType LocationType) *LocationSpec {
	for _, location := range locations {
		if location.LocationType == locationType {
			return &location
		}
	}
	return nil
}

func FindAllLocations(locations []LocationSpec, locationType LocationType) []LocationSpec {
	result := make([]LocationSpec, 0)
	for _, location := range locations {
		if location.LocationType == locationType {
			result = append(result, location)
		}
	}
	return result
}

func ValidateVersionConstraint(constraintStr string) *field.Error {
	// If no constraint is provided, any version is good
	if constraintStr == "" {
		return nil
	}

	constraints, err := version.ParseConstraints(constraintStr)
	if err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("requiresOperatorVersion"),
			constraintStr, err.Error(),
		)
	}

	// This check will force all operator builds to contain a valid version if it is expected
	// to handle spec.requiresOperatorVersion field properly.
	currentVersion := version.GetParsedVersion()
	ok, errors := constraints.Validate(currentVersion)
	if ok {
		return nil
	}

	return field.Forbidden(
		field.NewPath("spec").Child("requiresOperatorVersion"),
		fmt.Sprintf("current operator version %q does not satisfy the spec version constraint: %v", currentVersion.Original(), errors),
	)
}
