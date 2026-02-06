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
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"
)

func ValidateVersionConstraint(constraintStr string) *field.Error {
	// If no constraint is provided, any version is good
	if constraintStr == "" {
		return nil
	}

	path := field.NewPath("spec").Child("requiresOperatorVersion")
	constraints, err := version.ParseConstraints(constraintStr)
	if err != nil {
		return field.Invalid(path, constraintStr, err.Error())
	}

	// This check will force all operator builds to contain a valid version if it is expected
	// to handle spec.requiresOperatorVersion field properly.
	currentVersion := version.GetParsedVersion()
	ok, errors := constraints.Validate(currentVersion)
	if ok {
		return nil
	}

	details := fmt.Sprintf("current operator version %q does not satisfy the spec version constraint: %v", currentVersion.Original(), errors)
	return field.Forbidden(path, details)
}
