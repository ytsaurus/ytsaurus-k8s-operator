package webhooks

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

var _ = Describe("YtsaurusUpdatePlan webhook", func() {
	It("validates selectors, duplicate plans, and immutable cluster references", func() {
		plan := &ytv1.YtsaurusUpdatePlan{
			ObjectMeta: metav1.ObjectMeta{Name: "one-shot", Namespace: "default"},
			Spec: ytv1.YtsaurusUpdatePlanSpec{
				Ytsaurus:   ytv1.YtsaurusReference{Name: "future-cluster"},
				OneShot:    true,
				UpdatePlan: ytv1.UpdatePlan{{}},
			},
		}
		Expect(k8sClient.Create(ctx, plan)).To(MatchError(ContainSubstring("Either component or class")))
		plan.Spec.UpdatePlan = ytv1.UpdatePlan{{Class: ytv1.ComponentClassStateless}}
		Expect(k8sClient.Create(ctx, plan)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, plan)).To(Succeed()) })
		duplicate := plan.DeepCopy()
		duplicate.Name = "duplicate"
		duplicate.ResourceVersion = ""
		duplicate.UID = ""
		Expect(k8sClient.Create(ctx, duplicate)).To(MatchError(ContainSubstring("Duplicate value")))
		changed := plan.DeepCopy()
		changed.Spec.Ytsaurus.Name = "other"
		Expect(k8sClient.Update(ctx, changed)).To(MatchError(ContainSubstring("Ytsaurus reference is immutable")))
		plan.Spec.OneShot = false
		Expect(k8sClient.Update(ctx, plan)).To(Succeed())
	})

	It("accepts the sample after clearing the inline plan and rejects conflicting inline plans", func() {
		builder := &testutil.YtsaurusBuilder{Images: testutil.CurrImages, Namespace: "default"}
		builder.CreateMinimal()
		parent := builder.Ytsaurus
		parent.Name = "update-plan-cluster"
		inlinePlan := ytv1.UpdatePlan{{Class: ytv1.ComponentClassEverything}}
		parent.Spec.UpdatePlan = inlinePlan
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, parent)).To(Succeed()) })

		data, err := os.ReadFile(filepath.Join("..", "..", "config", "samples", "cluster_v1_ytsaurusupdateplan.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var plan ytv1.YtsaurusUpdatePlan
		Expect(yaml.UnmarshalStrict(data, &plan)).To(Succeed())
		Expect(plan.Spec.UpdatePlan).To(Equal(inlinePlan))
		plan.Namespace = parent.Namespace
		plan.Spec.Ytsaurus.Name = parent.Name
		Expect(k8sClient.Create(ctx, &plan)).To(MatchError(ContainSubstring("already has an inline update plan")))

		parent.Spec.UpdatePlan = nil
		Expect(k8sClient.Update(ctx, parent)).To(Succeed())
		Expect(k8sClient.Create(ctx, &plan)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, &plan)).To(Succeed()) })
		plan.Spec.OneShot = false
		Expect(k8sClient.Update(ctx, &plan)).To(Succeed())

		parent.Spec.UpdatePlan = inlinePlan
		Expect(k8sClient.Update(ctx, parent)).To(MatchError(ContainSubstring("already has an inline update plan")))
	})
})
