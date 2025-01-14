package clusteroperator

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openshift/cluster-capi-operator/pkg/controllers"
	"github.com/openshift/cluster-capi-operator/pkg/operatorstatus"
	"github.com/openshift/cluster-capi-operator/pkg/test"
)

var (
	operatorImageName                 = "cluster-kube-cluster-api-operator"
	operatorImageSource               = "test.com/operator:tag"
	kubeRBACProxyImageName            = "kube-rbac-proxy"
	kubeRBACProxySource               = "test.com/kube-rbac-proxy:tag"
	coreProviderImageName             = "cluster-capi-controllers" // nolint:gosec
	coreProviderImageSource           = "test.com/cluster-api:tag"
	infrastructureProviderImageName   = "aws-cluster-api-controllers"
	infrastructureProviderImageSource = "test.com/cluster-api-provider-aws:tag"
)

var _ = Describe("Reconcile components", func() {
	var r *ClusterOperatorReconciler

	ctx := context.Background()
	providerSpec := operatorv1.ProviderSpec{
		Version: "v1.0.0",
		Deployment: &operatorv1.DeploymentSpec{
			Containers: []operatorv1.ContainerSpec{
				{
					Name:     "manager",
					ImageURL: ptr.To("image.com/test:tag"),
				},
			},
		},
	}

	BeforeEach(func() {
		r = &ClusterOperatorReconciler{
			ClusterOperatorStatusClient: operatorstatus.ClusterOperatorStatusClient{
				Client: cl,
			},
			Images: map[string]string{
				operatorImageName:               operatorImageSource,
				kubeRBACProxyImageName:          kubeRBACProxySource,
				coreProviderImageName:           coreProviderImageSource,
				infrastructureProviderImageName: infrastructureProviderImageSource,
			},
		}
	})

	Context("reconcile core provider", func() { // nolint:dupl
		var coreProvider *operatorv1.CoreProvider

		BeforeEach(func() {
			coreProvider = &operatorv1.CoreProvider{
				TypeMeta: metav1.TypeMeta{
					Kind: "CoreProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-api",
					Namespace: controllers.DefaultManagedNamespace,
				},
				Spec: operatorv1.CoreProviderSpec{
					ProviderSpec: providerSpec,
				},
			}
		})

		AfterEach(func() {
			Expect(cl.Get(ctx, client.ObjectKey{
				Name:      coreProvider.Name,
				Namespace: coreProvider.Namespace,
			}, coreProvider)).To(Succeed())
			Expect(coreProvider.Spec.ProviderSpec.Deployment.Containers).To(HaveLen(1))
			Expect(coreProvider.Spec.ProviderSpec.Deployment.Containers[0].ImageURL).To(HaveValue(Equal(coreProviderImageSource)))

			Expect(test.CleanupAndWait(ctx, cl, coreProvider)).To(Succeed())
		})

		It("should create core provider and modify container images", func() {
			Expect(r.reconcileCoreProvider(ctx, coreProvider)).To(Succeed())
		})

		It("should update an existing core provider", func() {
			Expect(cl.Create(ctx, coreProvider)).To(Succeed())
			coreProvider.TypeMeta.Kind = "CoreProvider" // kind gets erased after Create()
			coreProvider.Spec.Version = "v2.0.0"
			Expect(r.reconcileCoreProvider(ctx, coreProvider)).To(Succeed())
			Expect(coreProvider.Spec.Version).To(Equal("v2.0.0"))
		})
	})

	Context("reconcile infrastructure provider", func() { // nolint:dupl
		var infraProvider *operatorv1.InfrastructureProvider

		BeforeEach(func() {
			infraProvider = &operatorv1.InfrastructureProvider{
				TypeMeta: metav1.TypeMeta{
					Kind: "InfrastructureProvider",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aws",
					Namespace: controllers.DefaultManagedNamespace,
				},
				Spec: operatorv1.InfrastructureProviderSpec{
					ProviderSpec: providerSpec,
				},
			}
		})

		AfterEach(func() {
			Expect(cl.Get(ctx, client.ObjectKey{
				Name:      infraProvider.Name,
				Namespace: infraProvider.Namespace,
			}, infraProvider)).To(Succeed())
			Expect(infraProvider.Spec.ProviderSpec.Deployment.Containers).To(HaveLen(1))
			Expect(infraProvider.Spec.ProviderSpec.Deployment.Containers[0].ImageURL).To(HaveValue(Equal(infrastructureProviderImageSource)))

			Expect(test.CleanupAndWait(ctx, cl, infraProvider)).To(Succeed())
		})

		It("should create infra provider and modify container images", func() {
			Expect(r.reconcileInfrastructureProvider(ctx, infraProvider)).To(Succeed())
		})

		It("should update an existing infra provider", func() {
			Expect(cl.Create(ctx, infraProvider)).To(Succeed())
			infraProvider.TypeMeta.Kind = "InfrastructureProvider" // kind gets erased after Create()
			infraProvider.Spec.Version = "v2.0.0"
			Expect(r.reconcileInfrastructureProvider(ctx, infraProvider)).To(Succeed())
			Expect(infraProvider.Spec.Version).To(Equal("v2.0.0"))
		})
	})

	Context("reconcile configmap", func() {
		var cm *corev1.ConfigMap

		BeforeEach(func() {
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-api-operator",
					Namespace: controllers.DefaultManagedNamespace,
					Labels:    map[string]string{"foo": "bar"},
				},
				Data: map[string]string{"foo": "bar"},
			}
		})

		AfterEach(func() {
			Expect(test.CleanupAndWait(ctx, cl, cm)).To(Succeed())
		})

		It("should create a configmap", func() {
			Expect(r.reconcileConfigMap(ctx, cm)).To(Succeed())
			Expect(cl.Get(ctx, client.ObjectKey{
				Name:      cm.Name,
				Namespace: cm.Namespace,
			}, cm)).To(Succeed())
			Expect(cm.Labels).To(HaveKeyWithValue("foo", "bar"))
			Expect(cm.Data).To(HaveKeyWithValue("foo", "bar"))
		})

		It("should update an existing deployment", func() {
			Expect(cl.Create(ctx, cm)).To(Succeed())
			cm.Labels = map[string]string{"foo": "baz"}
			cm.Data = map[string]string{"foo": "baz"}
			Expect(r.reconcileConfigMap(ctx, cm)).To(Succeed())
			Expect(cl.Get(ctx, client.ObjectKey{
				Name:      cm.Name,
				Namespace: cm.Namespace,
			}, cm)).To(Succeed())
			Expect(cm.Labels).To(HaveKeyWithValue("foo", "baz"))
			Expect(cm.Data).To(HaveKeyWithValue("foo", "baz"))
		})
	})
})

var _ = Describe("Container customization for provider", func() {
	reconciler := &ClusterOperatorReconciler{
		Images: map[string]string{
			kubeRBACProxyImageName:          kubeRBACProxySource,
			coreProviderImageName:           coreProviderImageSource,
			infrastructureProviderImageName: infrastructureProviderImageSource,
		},
	}

	It("should customize the container for core provider", func() {
		containers := reconciler.containerCustomizationFromProvider(
			"CoreProvider",
			"cluster-api",
			[]operatorv1.ContainerSpec{
				{
					Name: "manager",
				},
			})
		Expect(containers).To(HaveLen(1))
		Expect(containers[0].Name).To(Equal("manager"))
		Expect(containers[0].ImageURL).To(HaveValue(Equal("test.com/cluster-api:tag")))
	})
	It("should customize the container for infra provider with proxy", func() {
		containers := reconciler.containerCustomizationFromProvider(
			"InfrastructureProvider",
			"aws",
			[]operatorv1.ContainerSpec{
				{
					Name: "manager",
				},
				{
					Name: "kube-rbac-proxy",
				},
			})

		Expect(containers).To(HaveLen(2))
		Expect(containers[0].Name).To(Equal("manager"))
		Expect(containers[0].ImageURL).To(HaveValue(Equal("test.com/cluster-api-provider-aws:tag")))

		Expect(containers[1].Name).To(Equal("kube-rbac-proxy"))
		Expect(containers[1].ImageURL).To(HaveValue(Equal("test.com/kube-rbac-proxy:tag")))
	})
})
