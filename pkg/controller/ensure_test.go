package controller

import (
	"context"
	"io"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/core/v1alpha1"
	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/schema"
)

func TestEnsure(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Running Test Suite")
}

var _ = Describe("EnsureRunner", func() {
	logrus.SetOutput(io.Discard)

	ctx := context.TODO()

	var (
		cc            client.Client
		configuration *terraformv1alpha1.Configuration
		ensure        EnsureRunner
	)

	BeforeEach(func() {
		configuration = &terraformv1alpha1.Configuration{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "c1",
				Generation: 2,
			},
		}
		cc = fake.NewClientBuilder().
			WithScheme(schema.GetScheme()).
			WithStatusSubresource(&terraformv1alpha1.Configuration{}).
			WithRuntimeObjects(configuration).
			Build()
	})

	When("Run is called", func() {
		When("resource is reconciled for the first time", func() {
			It("sets initial reconcile status", func() {
				result, err := ensure.Run(ctx, cc, configuration, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				Expect(configuration.Status.CommonStatus.LastReconcile).NotTo(BeNil())
				Expect(configuration.Status.CommonStatus.LastReconcile.Generation).To(Equal(int64(2)))
				Expect(configuration.Status.CommonStatus.LastReconcile.Time).NotTo(BeZero())

				Expect(configuration.Status.CommonStatus.LastSuccess).NotTo(BeNil())
				Expect(configuration.Status.CommonStatus.LastSuccess.Generation).To(Equal(int64(2)))
				Expect(configuration.Status.CommonStatus.LastSuccess.Time).NotTo(BeZero())
			})
		})

		When("resource already reconciled", func() {
			var originalTime metav1.Time
			BeforeEach(func() {
				originalTime = metav1.Time{Time: time.Now().Add(-time.Hour).Truncate(time.Second)}
				configuration.Status = terraformv1alpha1.ConfigurationStatus{
					CommonStatus: corev1alpha1.CommonStatus{
						LastReconcile: &corev1alpha1.LastReconcileStatus{
							Generation: 2,
							Time:       originalTime,
						},
						LastSuccess: &corev1alpha1.LastReconcileStatus{
							Generation: 2,
							Time:       originalTime,
						},
					},
				}
				cc = fake.NewClientBuilder().
					WithScheme(schema.GetScheme()).
					WithStatusSubresource(&terraformv1alpha1.Configuration{}).
					WithRuntimeObjects(configuration).
					Build()
			})

			When("resource is unchanged", func() {
				It("does not update status", func() {
					result, err := ensure.Run(ctx, cc, configuration, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))

					Expect(configuration.Status.CommonStatus.LastReconcile).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastReconcile.Time).To(Equal(originalTime))

					Expect(configuration.Status.CommonStatus.LastSuccess).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastSuccess.Time).To(Equal(originalTime))
				})
			})

			When("resource generation has changed", func() {
				BeforeEach(func() {
					configuration.Generation = 9
					cc = fake.NewClientBuilder().
						WithScheme(schema.GetScheme()).
						WithStatusSubresource(&terraformv1alpha1.Configuration{}).
						WithRuntimeObjects(configuration).
						Build()
				})

				It("updates the status", func() {
					result, err := ensure.Run(ctx, cc, configuration, nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))

					Expect(configuration.Status.CommonStatus.LastReconcile).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastReconcile.Generation).To(Equal(int64(9)))
					Expect(configuration.Status.CommonStatus.LastReconcile.Time).NotTo(Equal(originalTime))

					Expect(configuration.Status.CommonStatus.LastSuccess).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastSuccess.Generation).To(Equal(int64(9)))
					Expect(configuration.Status.CommonStatus.LastSuccess.Time).NotTo(Equal(originalTime))
				})
			})

			When("ensurefunc updates resource", func() {
				It("updates the status", func() {
					result, err := ensure.Run(ctx, cc, configuration, []EnsureFunc{
						func(ctx context.Context) (reconcile.Result, error) {
							configuration.Spec.Module = "m1"
							return reconcile.Result{}, nil
						},
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))

					Expect(configuration.Status.CommonStatus.LastReconcile).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastReconcile.Time).NotTo(Equal(originalTime))

					Expect(configuration.Status.CommonStatus.LastSuccess).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastSuccess.Time).NotTo(Equal(originalTime))
				})
			})

			When("ensurefunc changes resource and requests requeue", func() {
				It("updates only last reconcile", func() {
					result, err := ensure.Run(ctx, cc, configuration, []EnsureFunc{
						func(ctx context.Context) (reconcile.Result, error) {
							configuration.Spec.Module = "m1"
							return reconcile.Result{Requeue: true}, nil
						},
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{Requeue: true}))

					Expect(configuration.Status.CommonStatus.LastReconcile).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastReconcile.Time).NotTo(Equal(originalTime))

					Expect(configuration.Status.CommonStatus.LastSuccess).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastSuccess.Time).To(Equal(originalTime))
				})
			})

			When("ensurefunc does not change resource but requests requeue", func() {
				It("does not update status", func() {
					result, err := ensure.Run(ctx, cc, configuration, []EnsureFunc{
						func(ctx context.Context) (reconcile.Result, error) {
							return reconcile.Result{Requeue: true}, nil
						},
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{Requeue: true}))

					Expect(configuration.Status.CommonStatus.LastReconcile).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastReconcile.Time).To(Equal(originalTime))

					Expect(configuration.Status.CommonStatus.LastSuccess).NotTo(BeNil())
					Expect(configuration.Status.CommonStatus.LastSuccess.Time).To(Equal(originalTime))
				})
			})
		})
	})
})
