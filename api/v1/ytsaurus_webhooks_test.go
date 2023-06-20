package v1

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test for Ytsaurus webhooks", func() {
	const namespace string = "default"

	Context("When setting up the test environment", func() {
		It("Should not accept Ytsaurus spec", func() {
			By("Creating a Ytsaurus resource without `default` http proxy role")

			ytsaurus := CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.HTTPProxies = []HTTPProxiesSpec{
				{
					Role: "not_default",
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("HTTP proxy with `default` role should exist")))

			By("Creating a Ytsaurus resource with the same RPC proxies roles")

			ytsaurus = CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.RPCProxies = []RPCProxiesSpec{
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.rpcProxies.role: Duplicate value: \"default\"")))

			By("Creating a Ytsaurus resource with the same HTTP proxies roles")

			ytsaurus = CreateBaseYtsaurusResource(namespace)

			ytsaurus.Spec.HTTPProxies = []HTTPProxiesSpec{
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			}

			Expect(k8sClient.Create(ctx, ytsaurus)).Should(MatchError(ContainSubstring("spec.httpProxies.role: Duplicate value: \"default\"")))

		})
	})
})
