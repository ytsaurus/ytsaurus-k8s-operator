package flows

import (
	"errors"
)

func FlattenSteps(items ...interface{}) ([]StepType, error) {
	var result []StepType
	for _, item := range items {
		switch val := item.(type) {
		case StepType:
			result = append(result, val)
		case []StepType:
			result = append(result, val...)
		default:
			return []StepType{}, errors.New("ConcatSteps expect only StepType or []StepType in arguments")
		}
	}
	return result, nil
}
