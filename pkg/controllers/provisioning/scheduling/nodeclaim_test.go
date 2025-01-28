package scheduling

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/karpenter/pkg/cloudprovider"
)

func TestFitsResourceClaims(t *testing.T) {
	RegisterFailHandler(Fail)
	g := NewWithT(t)

	instanceType := &cloudprovider.InstanceType{
		Name: "test-instance",
		ResourceSlices: map[string]cloudprovider.ResourceSlice{
			"gpu-vendor-a": {
				ResourceClass: "gpu-vendor-a",
				Resources: corev1.ResourceList{
					"vendor-a.com/gpu": resource.MustParse("1"),
				},
				Attributes: map[string]string{
					"vendor": "vendor-a",
					"model":  "t4",
					"memory": "16Gi",
				},
			},
		},
	}

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with no claims",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{},
			},
			expected: true,
		},
		{
			name: "pod with matching claim",
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
			name: "pod with non-matching claim",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					ResourceClaims: []corev1.PodResourceClaim{
						{
							Name: "gpu-vendor-b",
							Source: corev1.ResourceClaimSource{
								ResourceClaimName: "gpu-claim",
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with matching CEL expression",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := fitsResourceClaims(instanceType, tc.pod)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}
