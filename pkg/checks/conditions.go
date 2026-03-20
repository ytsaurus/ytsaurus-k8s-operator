package checks

import (
	"fmt"

	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeCondition(
	conditionType string,
	ok bool,
	msg string,
	err error,
) metav1.Condition {
	condition := metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionTrue,
		Reason:  "OK",
		Message: msg,
	}
	if err != nil {
		condition.Status = metav1.ConditionUnknown
		condition.Reason = "Error"
		condition.Message = msg + " Error: " + err.Error()
	} else if !ok {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "Failure"
	}
	return condition
}

func MergeConditions(
	output metav1.Condition,
	getter func(conditionType string) *metav1.Condition,
	inputs ...string,
) metav1.Condition {
	for _, input := range inputs {
		cond := ptr.Deref(getter(input), metav1.Condition{
			Type:    input,
			Status:  metav1.ConditionUnknown,
			Reason:  "Unknown",
			Message: "Status not found",
		})
		if cond.Status != metav1.ConditionTrue {
			if cond.Status == metav1.ConditionFalse || output.Status == metav1.ConditionTrue {
				output.Status = cond.Status
				output.Reason = cond.Reason
			}
			output.Message += fmt.Sprintf("%v %v: %v\n", cond.Type, cond.Reason, cond.Message)
		}
	}
	return output
}
