package webhooks

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

var _ = Describe("Separate node and proxy webhooks", func() {
	It("validates parent changes against separate components", func() {
		builder := &testutil.YtsaurusBuilder{Images: testutil.CurrImages, Namespace: "default"}
		builder.CreateMinimal()
		builder.WithBaseComponents()
		parent := builder.Ytsaurus
		execSpec := parent.Spec.ExecNodes[0]
		parent.Spec.ExecNodes = nil
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, parent)).To(Succeed())
		})
		nodes := &ytv1.YtsaurusExecNodes{
			ObjectMeta: metav1.ObjectMeta{Name: "separate-exec-nodes", Namespace: parent.Namespace},
			Spec: ytv1.YtsaurusExecNodesSpec{
				Ytsaurus:      ytv1.YtsaurusReference{Name: parent.Name},
				ExecNodesSpec: execSpec,
			},
		}
		Expect(k8sClient.Create(ctx, nodes)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, nodes)).To(Succeed())
		})

		parent.Spec.Schedulers = nil
		parent.Spec.ControllerAgents = nil
		err := k8sClient.Update(ctx, parent)
		Expect(apierrors.IsInvalid(err)).To(BeTrueBecause("separate exec nodes still require schedulers: %v", err))
		Expect(err).To(MatchError(ContainSubstring("spec.schedulers")))
	})

	It("validates creation, updates and conflicts for every resource type", func() {
		builder := &testutil.YtsaurusBuilder{Images: testutil.CurrImages, Namespace: "default"}
		builder.CreateMinimal()
		builder.WithBaseComponents()
		builder.WithRPCProxies()
		parent := builder.Ytsaurus
		Expect(k8sClient.Create(ctx, parent)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, parent)).To(Succeed())
		})

		builder.Ytsaurus = parent.DeepCopy()
		builder.Ytsaurus.Spec.DataNodes[0].Name = "attached"
		builder.Ytsaurus.Spec.ExecNodes[0].Name = "attached"
		builder.Ytsaurus.Spec.TabletNodes[0].Name = "attached"
		builder.Ytsaurus.Spec.HTTPProxies[0].Role = "attached"
		builder.Ytsaurus.Spec.RPCProxies[0].Role = "attached"
		resources := builder.SeparateComponentSpecs()
		Expect(resources).To(HaveLen(5))
		for _, obj := range resources {
			By(fmt.Sprintf("checking %T", obj))
			invalid := obj.DeepCopyObject().(client.Object)
			data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(invalid)
			Expect(err).NotTo(HaveOccurred())
			data["spec"].(map[string]interface{})["podLabels"] = map[string]interface{}{"invalid key": "value"}
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(data, invalid)).To(Succeed())
			err = k8sClient.Create(ctx, invalid)
			Expect(apierrors.IsInvalid(err)).To(BeTrueBecause("invalid pod labels must be rejected: %v", err))
			Expect(err).To(MatchError(ContainSubstring("spec.podLabels")))

			Expect(k8sClient.Create(ctx, obj)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
			})
			obj.SetLabels(map[string]string{"test": "updated"})
			Expect(k8sClient.Update(ctx, obj)).To(Succeed())

			invalid.SetResourceVersion(obj.GetResourceVersion())
			err = k8sClient.Update(ctx, invalid)
			Expect(apierrors.IsInvalid(err)).To(BeTrueBecause("invalid updates must be rejected: %v", err))
			Expect(err).To(MatchError(ContainSubstring("spec.podLabels")))

			duplicate := obj.DeepCopyObject().(client.Object)
			duplicate.SetName(obj.GetName() + "-duplicate")
			duplicate.SetResourceVersion("")
			duplicate.SetUID("")
			err = k8sClient.Create(ctx, duplicate)
			Expect(apierrors.IsInvalid(err)).To(BeTrueBecause("duplicate names and roles must be rejected: %v", err))
			Expect(err).To(MatchError(ContainSubstring("Duplicate value")))
		}
	})
})
