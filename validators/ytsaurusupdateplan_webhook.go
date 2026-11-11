package validators

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytcluster"
)

//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurusupdateplan,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurusupdateplan,verbs=create;update,versions=v1,name=vytsaurusupdateplan.kb.io,admissionReviewVersions=v1

func NewYtsaurusUpdatePlanValidator() *customValidator[*ytv1.YtsaurusUpdatePlan] {
	r := &customValidator[*ytv1.YtsaurusUpdatePlan]{Object: &ytv1.YtsaurusUpdatePlan{}}
	r.Validate = func(ctx context.Context, plan, oldPlan *ytv1.YtsaurusUpdatePlan) (admission.Warnings, error) {
		if plan == nil {
			return nil, nil
		}
		path := field.NewPath("spec", "updatePlan")
		errors := (&baseValidator{}).validateUpdatePlan(ytcluster.NewCluster(&ytv1.Ytsaurus{}), plan.Spec.UpdatePlan, path)
		parentPath := field.NewPath("spec", "ytsaurus", "name")
		if plan.GetYtsaurusName() == "" {
			errors = append(errors, field.Required(parentPath, "Ytsaurus name must be specified"))
		} else {
			for _, message := range validation.NameIsDNSSubdomain(plan.GetYtsaurusName(), false) {
				errors = append(errors, field.Invalid(parentPath, plan.GetYtsaurusName(), message))
			}
		}
		if len(errors) != 0 {
			return nil, apierrors.NewInvalid(ytv1.GroupVersion.WithKind("YtsaurusUpdatePlan").GroupKind(), plan.Name, errors)
		}

		var plans ytv1.YtsaurusUpdatePlanList
		if err := r.APIReader.List(ctx, &plans, client.InNamespace(plan.Namespace)); err != nil {
			return nil, err
		}
		for i := range plans.Items {
			other := &plans.Items[i]
			if other.Name != plan.Name && other.GetYtsaurusName() == plan.GetYtsaurusName() {
				errors = append(errors, field.Duplicate(parentPath, plan.GetYtsaurusName()))
			}
		}

		if len(errors) == 0 {
			clusterErrors, err := validateYtsaurusComponent(ctx, r, plan, oldPlan, "")
			if err != nil {
				return nil, err
			}
			errors = append(errors, clusterErrors...)
		}
		if len(errors) != 0 {
			return nil, apierrors.NewInvalid(ytv1.GroupVersion.WithKind("YtsaurusUpdatePlan").GroupKind(), plan.Name, errors)
		}
		return nil, nil
	}
	return r
}
