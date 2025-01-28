package scheduling

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/test"
)

func TestResourceClaimAllocation(t *testing.T) {
	RegisterFailHandler(Fail)
	g := NewWithT(t)

	ctx := context.Background()

	// Create a test NodePool
	nodePool := &v1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-nodepool",
		},
		Spec: v1.NodePoolSpec{
			Template: v1.NodeClaimTemplate{
				Spec: v1.NodeClaimSpec{},
			},
		},
	}

	// Create test instance types with ResourceSlices
	instanceTypes := []*cloudprovider.InstanceType{
		{
			Name: "gpu-instance",
			ResourceSlices: map[string]cloudprovider.ResourceSlice{
				"gpu-vendor-a": {
					ResourceClass: "gpu-vendor-a",
					Resources: corev1.ResourceList{
						"vendor-a.com/gpu": resource.MustParse("4"),
					},
					Attributes: map[string]string{
						"vendor": "vendor-a",
						"model":  "t4",
						"memory": "16Gi",
					},
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("32Gi"),
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with single gpu claim",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					ResourceClaims: []corev1.PodResourceClaim{
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim",
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with multiple gpu claims that fit",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					ResourceClaims: []corev1.PodResourceClaim{
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim-1",
							},
						},
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim-2",
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with gpu claims that exceed capacity",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					ResourceClaims: []corev1.PodResourceClaim{
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim-1",
							},
						},
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim-2",
							},
						},
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim-3",
							},
						},
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim-4",
							},
						},
						{
							Name: "gpu-vendor-a",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim-5",
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nodeClaim := NewNodeClaim(nodePool, instanceTypes)
			err := nodeClaim.trySchedulePod(ctx, tc.pod)
			if tc.expected {
				g.Expect(err).To(BeNil())
			} else {
				g.Expect(err).ToNot(BeNil())
			}
		})
	}
}
