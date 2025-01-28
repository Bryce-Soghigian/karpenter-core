package test

import (
	"fmt"

	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type ResourceClassOptions struct {
	Name       string
	DriverName string
	Parameters map[string]string
}

func ResourceClass(options ResourceClassOptions) *resourcev1alpha2.ResourceClass {
	if options.Name == "" {
		options.Name = fmt.Sprintf("resource-class-%s", uuid.NewUUID())
	}
	if options.DriverName == "" {
		options.DriverName = "test-driver"
	}
	return &resourcev1alpha2.ResourceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.Name,
		},
		DriverName: options.DriverName,
		Parameters: options.Parameters,
	}
}

type ResourceClaimOptions struct {
	Name              string
	Namespace         string
	ResourceClassName string
	AllocationMode    string
	Resources         []resourcev1alpha2.ResourceClaimRequest
}

func ResourceClaim(options ResourceClaimOptions) *resourcev1alpha2.ResourceClaim {
	if options.Name == "" {
		options.Name = fmt.Sprintf("resource-claim-%s", uuid.NewUUID())
	}
	if options.Namespace == "" {
		options.Namespace = "default"
	}
	return &resourcev1alpha2.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: options.Namespace,
		},
		Spec: resourcev1alpha2.ResourceClaimSpec{
			ResourceClassName: options.ResourceClassName,
			AllocationMode:    options.AllocationMode,
			Resources:         options.Resources,
		},
	}
}
