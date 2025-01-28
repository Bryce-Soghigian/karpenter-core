package integration

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/karpenter/kwok"
	"sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/test"
)

var _ = Describe("DRA", func() {
	var nodePool *v1.NodePool
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
		nodePool = test.NodePool(v1.NodePool{
			Spec: v1.NodePoolSpec{
				Template: v1.NodeClaimTemplate{
					Spec: v1.NodeClaimSpec{
						Requirements: []v1.NodeSelectorRequirementWithMinValues{
							{
								NodeSelectorRequirement: corev1.NodeSelectorRequirement{
									Key:      corev1.LabelInstanceTypeStable,
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"gpu-instance-type-a", "gpu-instance-type-b"},
								},
							},
						},
					},
				},
			},
		})
	})

	Context("ResourceClaim Scheduling", func() {
		var resourceClass *resourcev1alpha2.ResourceClass
		var resourceClaim *resourcev1alpha2.ResourceClaim

		BeforeEach(func() {
			resourceClass = &resourcev1alpha2.ResourceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gpu-vendor-a",
				},
				DriverName: "kwok.sh/gpu",
				Parameters: map[string]string{
					"vendor": "vendor-a",
					"model":  "t4",
				},
			}
			Expect(env.Client.Create(ctx, resourceClass)).To(Succeed())

			resourceClaim = &resourcev1alpha2.ResourceClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gpu-claim",
					Namespace: "default",
				},
				Spec: resourcev1alpha2.ResourceClaimSpec{
					ResourceClassName: "gpu-vendor-a",
					AllocationMode:    "attributes.vendor == 'vendor-a' && attributes.model == 't4'",
					Resources: []resourcev1alpha2.ResourceClaimRequest{
						{
							Name: kwok.ResourceGPUVendorA,
							Resources: corev1.ResourceList{
								kwok.ResourceGPUVendorA: resource.MustParse("1"),
							},
						},
					},
				},
			}
			Expect(env.Client.Create(ctx, resourceClaim)).To(Succeed())
		})

		It("should schedule pod with matching ResourceClaim", func() {
			pod := test.Pod(test.PodOptions{
				ResourceClaims: []corev1.PodResourceClaim{
					{
						Name: "gpu-vendor-a",
						Source: corev1.ResourceClaimSource{
							ResourceClaimName: resourceClaim.Name,
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, pod)
			ExpectScheduled(ctx, env.Client, pod)
		})

		It("should not schedule pod with non-matching ResourceClaim", func() {
			resourceClaim.Spec.AllocationMode = "attributes.vendor == 'vendor-b'"
			Expect(env.Client.Update(ctx, resourceClaim)).To(Succeed())

			pod := test.Pod(test.PodOptions{
				ResourceClaims: []corev1.PodResourceClaim{
					{
						Name: "gpu-vendor-a",
						Source: corev1.ResourceClaimSource{
							ResourceClaimName: resourceClaim.Name,
						},
					},
				},
			})
			ExpectApplied(ctx, env.Client, nodePool, pod)
			ExpectNotScheduled(ctx, env.Client, pod)
		})

		It("should schedule multiple pods with ResourceClaims that fit capacity", func() {
			pods := []*corev1.Pod{}
			for i := 0; i < 4; i++ {
				pod := test.Pod(test.PodOptions{
					ResourceClaims: []corev1.PodResourceClaim{
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: resourceClaim.Name,
							},
						},
					},
				})
				pods = append(pods, pod)
			}
			ExpectApplied(ctx, env.Client, nodePool)
			for _, pod := range pods {
				ExpectApplied(ctx, env.Client, pod)
			}
			for _, pod := range pods {
				ExpectScheduled(ctx, env.Client, pod)
			}
		})

		It("should not schedule pods with ResourceClaims that exceed capacity", func() {
			pods := []*corev1.Pod{}
			for i := 0; i < 5; i++ {
				pod := test.Pod(test.PodOptions{
					ResourceClaims: []corev1.PodResourceClaim{
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: resourceClaim.Name,
							},
						},
					},
				})
				pods = append(pods, pod)
			}
			ExpectApplied(ctx, env.Client, nodePool)
			for _, pod := range pods {
				ExpectApplied(ctx, env.Client, pod)
			}
			scheduledCount := 0
			for _, pod := range pods {
				if pod.Spec.NodeName != "" {
					scheduledCount++
				}
			}
			Expect(scheduledCount).To(Equal(4)) // Only 4 GPUs available
		})
	})
})

func ExpectScheduled(ctx context.Context, client client.Client, pod *corev1.Pod) {
	Eventually(func() bool {
		if err := client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			return false
		}
		return pod.Spec.NodeName != ""
	}, 30*time.Second).Should(BeTrue())
}

func ExpectNotScheduled(ctx context.Context, client client.Client, pod *corev1.Pod) {
	Consistently(func() bool {
		if err := client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			return true
		}
		return pod.Spec.NodeName == ""
	}, 10*time.Second).Should(BeTrue())
}
