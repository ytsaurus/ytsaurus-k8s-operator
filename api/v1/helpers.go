package v1

import (
	"fmt"

	"github.com/hashicorp/go-version"
	opver "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

	constraint, err := version.NewConstraint(constraintStr)
	if err != nil {
		return field.Invalid(
			field.NewPath("spec").Child("requiresOperatorVersion"),
			constraintStr, err.Error(),
		)
	}

	// TODO: This requires some consideration.
	// Ideally we shouldn't fail if the operator is build with some random version string
	// that cannot be parsed, but if the spec contains a version lock then we also need to
	// provide the guarantee about that.
	// This check will force all operator builds to contain a valid version if it is expected
	// to handle spec.requiresOperatorVersion field properly.
	currentVersion, err := version.NewVersion(opver.GetVersion())
	if err != nil {
		return field.InternalError(field.NewPath("spec").Child("requiresOperatorVersion"), err)
	}

	if constraint.Check(currentVersion) {
		return nil
	}

	return field.Forbidden(
		field.NewPath("spec").Child("requiresOperatorVersion"),
		fmt.Sprintf("current operator build %q does not satisfy the spec version constraint %q", currentVersion.String(), constraintStr),
	)
}
