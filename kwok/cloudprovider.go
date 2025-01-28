package kwok

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "sigs.k8s.io/karpenter/pkg/apis/v1"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
)

const (
	ResourceGPUVendorA corev1.ResourceName = "kwok.sh/vendor-a-gpu"
	ResourceGPUVendorB corev1.ResourceName = "kwok.sh/vendor-b-gpu"
)

type CloudProvider struct {
	mu sync.RWMutex
}

func NewCloudProvider() *CloudProvider {
	return &CloudProvider{}
}

func (c *CloudProvider) GetInstanceTypes(_ context.Context, _ *v1.NodePool) ([]*cloudprovider.InstanceType, error) {
	return []*cloudprovider.InstanceType{
		{
			Name: "default-instance-type",
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, "default-instance-type"),
				scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
				scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, string(corev1.Linux)),
			),
			Offerings: []cloudprovider.Offering{
				{
					Requirements: scheduling.NewLabelRequirements(map[string]string{
						v1.CapacityTypeLabelKey:  "on-demand",
						corev1.LabelTopologyZone: "test-zone-1",
					}),
					Price:     1.0,
					Available: true,
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				corev1.ResourcePods:   resource.MustParse("100"),
			},
			Overhead: &cloudprovider.InstanceTypeOverhead{
				KubeReserved: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		},
		{
			Name: "gpu-instance-type-a",
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, "gpu-instance-type-a"),
				scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
				scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, string(corev1.Linux)),
			),
			Offerings: []cloudprovider.Offering{
				{
					Requirements: scheduling.NewLabelRequirements(map[string]string{
						v1.CapacityTypeLabelKey:  "on-demand",
						corev1.LabelTopologyZone: "test-zone-1",
					}),
					Price:     2.0,
					Available: true,
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("8"),
				corev1.ResourceMemory: resource.MustParse("32Gi"),
				corev1.ResourcePods:   resource.MustParse("100"),
				ResourceGPUVendorA:    resource.MustParse("4"),
			},
			ResourceSlices: map[string]cloudprovider.ResourceSlice{
				"gpu-vendor-a": {
					ResourceClass: "gpu-vendor-a",
					Resources: corev1.ResourceList{
						ResourceGPUVendorA: resource.MustParse("4"),
					},
					Attributes: map[string]string{
						"vendor": "vendor-a",
						"model":  "t4",
						"memory": "16Gi",
					},
				},
			},
			Overhead: &cloudprovider.InstanceTypeOverhead{
				KubeReserved: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			},
		},
		{
			Name: "gpu-instance-type-b",
			Requirements: scheduling.NewRequirements(
				scheduling.NewRequirement(corev1.LabelInstanceTypeStable, corev1.NodeSelectorOpIn, "gpu-instance-type-b"),
				scheduling.NewRequirement(corev1.LabelArchStable, corev1.NodeSelectorOpIn, "amd64"),
				scheduling.NewRequirement(corev1.LabelOSStable, corev1.NodeSelectorOpIn, string(corev1.Linux)),
			),
			Offerings: []cloudprovider.Offering{
				{
					Requirements: scheduling.NewLabelRequirements(map[string]string{
						v1.CapacityTypeLabelKey:  "on-demand",
						corev1.LabelTopologyZone: "test-zone-1",
					}),
					Price:     4.0,
					Available: true,
				},
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("16"),
				corev1.ResourceMemory: resource.MustParse("64Gi"),
				corev1.ResourcePods:   resource.MustParse("100"),
				ResourceGPUVendorB:    resource.MustParse("8"),
			},
			ResourceSlices: map[string]cloudprovider.ResourceSlice{
				"gpu-vendor-b": {
					ResourceClass: "gpu-vendor-b",
					Resources: corev1.ResourceList{
						ResourceGPUVendorB: resource.MustParse("8"),
					},
					Attributes: map[string]string{
						"vendor": "vendor-b",
						"model":  "a100",
						"memory": "80Gi",
					},
				},
			},
			Overhead: &cloudprovider.InstanceTypeOverhead{
				KubeReserved: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("200Mi"),
				},
			},
		},
	}, nil
}

// Rest of the CloudProvider implementation...
