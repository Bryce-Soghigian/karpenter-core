/*
Copyright The Kubernetes Authors.

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

package scheduling

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/samber/lo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/klog"
	"sigs.k8s.io/karpenter/pkg/cel"
	"sigs.k8s.io/karpenter/pkg/cloudprovider"
	"sigs.k8s.io/karpenter/pkg/scheduling"
	"sigs.k8s.io/karpenter/pkg/utils/resources"
)

// NodeClaim is a set of constraints, compatible pods, and possible instance types that could fulfill these constraints. This
// will be turned into one or more actual node instances within the cluster after bin packing.
type NodeClaim struct {
	NodeClaimTemplate

	Pods            []*v1.Pod
	topology        *Topology
	hostPortUsage   *scheduling.HostPortUsage
	daemonResources v1.ResourceList
	hostname        string
}

var nodeID int64

func NewNodeClaim(nodeClaimTemplate *NodeClaimTemplate, topology *Topology, daemonResources v1.ResourceList, instanceTypes []*cloudprovider.InstanceType) *NodeClaim {
	// Copy the template, and add hostname
	hostname := fmt.Sprintf("hostname-placeholder-%04d", atomic.AddInt64(&nodeID, 1))
	topology.Register(v1.LabelHostname, hostname)
	template := *nodeClaimTemplate
	template.Requirements = scheduling.NewRequirements()
	template.Requirements.Add(nodeClaimTemplate.Requirements.Values()...)
	template.Requirements.Add(scheduling.NewRequirement(v1.LabelHostname, v1.NodeSelectorOpIn, hostname))
	template.InstanceTypeOptions = instanceTypes
	template.Spec.Resources.Requests = daemonResources

	return &NodeClaim{
		NodeClaimTemplate: template,
		hostPortUsage:     scheduling.NewHostPortUsage(),
		topology:          topology,
		daemonResources:   daemonResources,
		hostname:          hostname,
	}
}

func (n *NodeClaim) Add(pod *v1.Pod, podRequests v1.ResourceList) error {
	// Check Taints
	if err := scheduling.Taints(n.Spec.Taints).Tolerates(pod); err != nil {
		return err
	}

	// exposed host ports on the node
	hostPorts := scheduling.GetHostPorts(pod)
	if err := n.hostPortUsage.Conflicts(pod, hostPorts); err != nil {
		return fmt.Errorf("checking host port usage, %w", err)
	}
	nodeClaimRequirements := scheduling.NewRequirements(n.Requirements.Values()...)
	podRequirements := scheduling.NewPodRequirements(pod)

	// Check NodeClaim Affinity Requirements
	if err := nodeClaimRequirements.Compatible(podRequirements, scheduling.AllowUndefinedWellKnownLabels); err != nil {
		return fmt.Errorf("incompatible requirements, %w", err)
	}
	nodeClaimRequirements.Add(podRequirements.Values()...)

	strictPodRequirements := podRequirements
	if scheduling.HasPreferredNodeAffinity(pod) {
		// strictPodRequirements is important as it ensures we don't inadvertently restrict the possible pod domains by a
		// preferred node affinity.  Only required node affinities can actually reduce pod domains.
		strictPodRequirements = scheduling.NewStrictPodRequirements(pod)
	}
	// Check Topology Requirements
	topologyRequirements, err := n.topology.AddRequirements(strictPodRequirements, nodeClaimRequirements, pod, scheduling.AllowUndefinedWellKnownLabels)
	if err != nil {
		return err
	}
	if err = nodeClaimRequirements.Compatible(topologyRequirements, scheduling.AllowUndefinedWellKnownLabels); err != nil {
		return err
	}
	nodeClaimRequirements.Add(topologyRequirements.Values()...)

	// Check instance type combinations
	requests := resources.Merge(n.Spec.Resources.Requests, podRequests)

	filtered := filterInstanceTypes(n.InstanceTypeOptions, nodeClaimRequirements, requests, n.Pods)

	if len(filtered.remaining) == 0 {
		// log the total resources being requested (daemonset + the pod)
		cumulativeResources := resources.Merge(n.daemonResources, podRequests)
		return fmt.Errorf("no instance type satisfied resources %s and requirements %s (%s)", resources.String(cumulativeResources), nodeClaimRequirements, filtered.FailureReason())
	}

	// Update node
	n.Pods = append(n.Pods, pod)
	n.InstanceTypeOptions = filtered.remaining
	n.Spec.Resources.Requests = requests
	n.Requirements = nodeClaimRequirements
	n.topology.Record(pod, nodeClaimRequirements, scheduling.AllowUndefinedWellKnownLabels)
	n.hostPortUsage.Add(pod, hostPorts)
	return nil
}

func (n *NodeClaim) Destroy() {
	n.topology.Unregister(v1.LabelHostname, n.hostname)
}

// FinalizeScheduling is called once all scheduling has completed and allows the node to perform any cleanup
// necessary before its requirements are used for instance launching
func (n *NodeClaim) FinalizeScheduling() {
	// We need nodes to have hostnames for topology purposes, but we don't want to pass that node name on to consumers
	// of the node as it will be displayed in error messages
	delete(n.Requirements, v1.LabelHostname)
}

func (n *NodeClaim) RemoveInstanceTypeOptionsByPriceAndMinValues(reqs scheduling.Requirements, maxPrice float64) (*NodeClaim, error) {
	n.InstanceTypeOptions = lo.Filter(n.InstanceTypeOptions, func(it *cloudprovider.InstanceType, _ int) bool {
		launchPrice := it.Offerings.Available().WorstLaunchPrice(reqs)
		return launchPrice < maxPrice
	})
	if _, err := n.InstanceTypeOptions.SatisfiesMinValues(reqs); err != nil {
		return nil, err
	}
	return n, nil
}

func InstanceTypeList(instanceTypeOptions []*cloudprovider.InstanceType) string {
	var itSb strings.Builder
	for i, it := range instanceTypeOptions {
		// print the first 5 instance types only (indices 0-4)
		if i > 4 {
			fmt.Fprintf(&itSb, " and %d other(s)", len(instanceTypeOptions)-i)
			break
		} else if i > 0 {
			fmt.Fprint(&itSb, ", ")
		}
		fmt.Fprint(&itSb, it.Name)
	}
	return itSb.String()
}

type filterResults struct {
	remaining                  []*cloudprovider.InstanceType
	incompatible               []*cloudprovider.InstanceType
	insufficientResources      []*cloudprovider.InstanceType
	insufficientResourceClaims []*cloudprovider.InstanceType
	noOffering                 []*cloudprovider.InstanceType
	minValuesIncompatibleErr   error
}

// FailureReason returns a presentable string explaining why all instance types were filtered out
//
//nolint:gocyclo
func (r filterResults) FailureReason() string {
	if len(r.remaining) > 0 {
		return ""
	}

	// minValues is specified in the requirements and is not met
	if r.minValuesIncompatibleErr != nil {
		return r.minValuesIncompatibleErr.Error()
	}

	// no instance type met any of the three criteria, meaning each criteria was enough to completely prevent
	// this pod from scheduling
	if len(r.incompatible) > 0 && len(r.insufficientResources) > 0 && len(r.noOffering) > 0 && len(r.insufficientResourceClaims) > 0 {
		return "no instance type met the scheduling requirements or had enough resources or had a required offering"
	}

	// check the other pairwise criteria
	if len(r.incompatible) > 0 && len(r.insufficientResources) > 0 {
		return "no instance type met the scheduling requirements or had enough resources"
	}

	if len(r.incompatible) > 0 && len(r.noOffering) > 0 {
		return "no instance type met the scheduling requirements or had a required offering"
	}

	if len(r.insufficientResources) > 0 && len(r.noOffering) > 0 {
		return "no instance type had enough resources or had a required offering"
	}

	// and then each individual criteria. These are sort of the same as above in that each one indicates that no
	// instance type matched that criteria at all, so it was enough to exclude all instance types.  I think it's
	// helpful to have these separate, since we can report the multiple excluding criteria above.
	if len(r.incompatible) > 0 {
		return "no instance type met all requirements"
	}

	if len(r.insufficientResources) > 0 {
		msg := "no instance type has enough resources"
		// special case for a user typo I saw reported once
		if r.requests.Cpu().Cmp(resource.MustParse("1M")) >= 0 {
			msg += " (CPU request >= 1 Million, m vs M typo?)"
		}
		return msg
	}

	if len(r.noOffering) > 0 {
		return "no instance type has the required offering"
	}

	// see if any pair of criteria was enough to exclude all instances
	if len(r.insufficientResourceClaims) > 0 {
		return "no instance type which met the scheduling requirements and had enough resources, had a required offering"
	}

	// finally all instances were filtered out, but we had at least one instance that met each criteria, and met each
	// pairwise set of criteria, so the only thing that remains is no instance which met all three criteria simultaneously
	return "no instance type met the requirements/resources/offering tuple"
}

//nolint:gocyclo
func filterInstanceTypes(instanceTypes []*cloudprovider.InstanceType, requirements scheduling.Requirements, requests v1.ResourceList, pods []*v1.Pod) filterResults {
	results := filterResults{}
	for _, it := range instanceTypes {
		itCompat := compatible(it, requirements)
		itFits := fits(it, requests)
		itHasOffering := it.Offerings.HasCompatible(requirements)
		itFitsResourceClaims := true

		// Check if instance type can satisfy all pod ResourceClaims
		for _, pod := range pods {
			if !fitsResourceClaims(it, pod) {
				itFitsResourceClaims = false
				break
			}
		}

		// Track incompatibilities for error reporting
		if !itCompat {
			results.incompatible = append(results.incompatible, it)
		}
		if !itFits {
			results.insufficientResources = append(results.insufficientResources, it)
		}
		if !itHasOffering {
			results.noOffering = append(results.noOffering, it)
		}
		if !itFitsResourceClaims {
			results.insufficientResourceClaims = append(results.insufficientResourceClaims, it)
		}

		// and if it met all criteria, we keep the instance type and continue filtering.  We now won't be reporting
		// any errors.
		if itCompat && itFits && itHasOffering && itFitsResourceClaims {
			results.remaining = append(results.remaining, it)
		}
	}

	if requirements.HasMinValues() {
		// We don't care about the minimum number of instance types that meet our requirements here, we only care if they meet our requirements.
		_, results.minValuesIncompatibleErr = results.remaining.SatisfiesMinValues(requirements)
		if results.minValuesIncompatibleErr != nil {
			// If minValues is NOT met for any of the requirement across InstanceTypes, then return empty InstanceTypeOptions as we cannot launch with the remaining InstanceTypes.
			results.remaining = nil
		}
	}
	return results
}

func compatible(instanceType *cloudprovider.InstanceType, requirements scheduling.Requirements) bool {
	return instanceType.Requirements.Intersects(requirements) == nil
}

func fits(instanceType *cloudprovider.InstanceType, requests v1.ResourceList) bool {
	return resources.Fits(requests, instanceType.Allocatable())
}

// fitsResourceClaims checks if the instance type can satisfy all ResourceClaims
// by evaluating the CEL expressions against the ResourceSlice attributes
func fitsResourceClaims(instanceType *cloudprovider.InstanceType, pod *v1.Pod) bool {
	// If pod has no ResourceClaims, it fits
	if len(pod.Spec.ResourceClaims) == 0 {
		return true
	}

	// If instance type has no ResourceSlices but pod has claims, it doesn't fit
	if len(instanceType.ResourceSlices) == 0 {
		return false
	}

	for _, claim := range pod.Spec.ResourceClaims {
		// Find matching ResourceSlice by ResourceClass
		slice, ok := instanceType.ResourceSlices[claim.Name]
		if !ok {
			return false
		}

		// Evaluate CEL expression from claim against slice attributes
		matches, err := cel.EvaluateResourceClaimSelector(claim.Spec.AllocationMode, slice.Attributes)
		if err != nil {
			// Log error but treat as non-matching for safety
			klog.ErrorS(err, "Failed to evaluate CEL expression",
				"pod", klog.KObj(pod),
				"claim", claim.Name,
				"instanceType", instanceType.Name)
			return false
		}
		if !matches {
			return false
		}

		// Check if resources are available
		if !resources.Fits(claim.Spec.Resources.Requests, slice.Resources) {
			return false
		}
	}
	return true
}
