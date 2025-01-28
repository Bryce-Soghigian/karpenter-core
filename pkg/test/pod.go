package test

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type PodOptions struct {
	Name                 string
	Namespace            string
	Image                string
	ResourceRequirements corev1.ResourceRequirements
	NodeSelector         map[string]string
	Labels               map[string]string
	Annotations          map[string]string
	Tolerations          []corev1.Toleration
	Affinity             *corev1.Affinity
	ResourceClaims       []corev1.PodResourceClaim
}

func Pod(options PodOptions) *corev1.Pod {
	if options.Name == "" {
		options.Name = fmt.Sprintf("pod-%s", uuid.NewUUID())
	}
	if options.Namespace == "" {
		options.Namespace = "default"
	}
	if options.Image == "" {
		options.Image = "public.ecr.aws/eks-distro/kubernetes/pause:3.7"
	}
	if options.ResourceRequirements.Requests == nil {
		options.ResourceRequirements.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		}
	}
	if options.ResourceRequirements.Limits == nil {
		options.ResourceRequirements.Limits = options.ResourceRequirements.Requests
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        options.Name,
			Namespace:   options.Namespace,
			Labels:      options.Labels,
			Annotations: options.Annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:      "pause",
				Image:     options.Image,
				Resources: options.ResourceRequirements,
			}},
			NodeSelector:   options.NodeSelector,
			Tolerations:    options.Tolerations,
			Affinity:       options.Affinity,
			ResourceClaims: options.ResourceClaims,
		},
	}
}
