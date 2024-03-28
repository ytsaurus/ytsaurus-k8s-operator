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

package controllers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ptr "k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	clusterv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var k8sClient client.Client
var ctx context.Context

func TestAPIs(t *testing.T) {
	if os.Getenv("YTSAURUS_ENABLE_E2E_TESTS") != "true" {
		t.Skip("skipping E2E tests: set YTSAURUS_ENABLE_E2E_TESTS environment variable to 'true'")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite")
}

var _ = SynchronizedBeforeSuite(func(ctx context.Context) []byte {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:        []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:    true,
		UseExistingCluster:       ptr.Bool(true),
		AttachControlPlaneOutput: true,
		Config:                   cfg,
	}

	_, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(testEnv.Stop)

	// Cannot serialize rest config here - just load again in each process and check host to be sure.
	return []byte(cfg.Host)
}, func(ctx context.Context, host []byte) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping k8s client")

	cfg, err := config.GetConfig()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg.Host).To(Equal(string(host)))

	err = clusterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = BeforeEach(func() {
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(context.TODO())
	DeferCleanup(func() {
		cancel()
		ctx = nil
	})
})
