package controllers

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

var _ = Describe("Basic test for YTsaurus controller", func() {

	const (
		namespace = "default"

		timeout  = time.Second * 90
		interval = time.Millisecond * 250
	)

	Context("When setting up the test environment", func() {
		It("Should run YTsaurus", func() {
			By("Creating a YTsaurus resource")
			ctx := context.Background()

			logger := log.FromContext(ctx)

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)

			defer func() {
				if err := k8sClient.Delete(ctx, ytsaurus); err != nil {
					logger.Error(err, "Deleting ytsaurus failed")
				}

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: v1.ObjectMeta{
						Name:      "master-data-ms-0",
						Namespace: namespace,
					},
				}

				if err := k8sClient.Delete(ctx, pvc); err != nil {
					logger.Error(err, "Deleting ytsaurus pvc failed")
				}
			}()

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())

			ytsaurusLookupKey := types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}

			Eventually(func() bool {
				createdYtsaurus := &ytv1.Ytsaurus{}
				err := k8sClient.Get(ctx, ytsaurusLookupKey, createdYtsaurus)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			By("Check pods are running")
			for _, podName := range []string{"ds-0", "ms-0", "hp-default-0", "dnd-0", "end-0"} {
				Eventually(func() bool {
					pod := &corev1.Pod{}
					err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, pod)
					if err != nil {
						return false
					}
					return pod.Status.Phase == corev1.PodRunning
				}, timeout, interval).Should(BeTrue())
			}

			By("Check ytsaurus state equal to `Running`")
			Eventually(func() bool {
				ytsaurus := &ytv1.Ytsaurus{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
				if err != nil {
					return false
				}
				return ytsaurus.Status.State == ytv1.ClusterStateRunning
			}, timeout*2, interval).Should(BeTrue())

			By("Run cluster update")

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)).Should(Succeed())
			ytsaurus.Spec.CoreImage = "ytsaurus/ytsaurus:dev"
			Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

			Eventually(func() bool {
				ytsaurus := &ytv1.Ytsaurus{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
				if err != nil {
					return false
				}
				return ytsaurus.Status.State == ytv1.ClusterStateUpdating
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				ytsaurus := &ytv1.Ytsaurus{}
				err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
				if err != nil {
					return false
				}
				return ytsaurus.Status.State == ytv1.ClusterStateRunning
			}, timeout, interval).Should(BeTrue())
		})
	})

})
