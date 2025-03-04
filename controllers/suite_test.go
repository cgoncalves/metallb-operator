/*


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

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	metallbv1beta1 "github.com/metallb/metallb-operator/api/v1beta1"
	// +kubebuilder:scaffold:imports
)

const (
	MetalLBTestNameSpace              = "metallb-test-namespace"
	MetalLBManifestPathControllerTest = "../bindata/deployment"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("Setting MetalLBReconcilier environment variables")
	Expect(os.Setenv("SPEAKER_IMAGE", "test-speaker:latest")).To(Succeed())
	Expect(os.Setenv("CONTROLLER_IMAGE", "test-controller:latest")).To(Succeed())
	Expect(os.Setenv("FRR_IMAGE", "test-frr:latest")).To(Succeed())
	Expect(os.Setenv("KUBE_RBAC_PROXY_IMAGE", "test-kube-rbac-proxy:latest")).To(Succeed())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = metallbv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	testNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: MetalLBTestNameSpace,
		},
	}

	err = k8sClient.Create(context.Background(), testNamespace)
	Expect(err).ToNot(HaveOccurred())

	ManifestPath = MetalLBManifestPathControllerTest // This is needed as the tests need to reference a directory backward
	PodMonitorsPath = fmt.Sprintf("%s/%s", MetalLBManifestPathControllerTest, "prometheus-operator")

	bgpType := os.Getenv("METALLB_BGP_TYPE")
	webHookEnabled, _ := strconv.ParseBool(os.Getenv("ENABLE_WEBHOOK"))
	err = (&MetalLBReconciler{
		Client:    k8sClient,
		Scheme:    scheme.Scheme,
		Log:       ctrl.Log.WithName("controllers").WithName("MetalLB"),
		Namespace: MetalLBTestNameSpace,
	}).SetupWithManager(k8sManager, bgpType, webHookEnabled)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	// restore Manifestpaths for both controller to their original value
	ManifestPath = MetalLBManifestPathController
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
