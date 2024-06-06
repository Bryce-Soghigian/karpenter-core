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

package disruption

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/karpenter/pkg/apis/v1beta1"
	"sigs.k8s.io/karpenter/pkg/controllers/provisioning/scheduling"
	"sigs.k8s.io/karpenter/pkg/metrics"
)

const SingleNodeConsolidationTimeoutDuration = 3 * time.Minute

// SingleNodeConsolidation is the consolidation controller that performs single-node consolidation.
type SingleNodeConsolidation struct {
	consolidation
}

func NewSingleNodeConsolidation(consolidation consolidation) *SingleNodeConsolidation {
	return &SingleNodeConsolidation{consolidation: consolidation}
}

// ComputeCommand generates a disruption command given candidates
// nolint:gocyclo
func (s *SingleNodeConsolidation) ComputeCommand(ctx context.Context, disruptionBudgetMapping map[string]map[v1beta1.DisruptionReason]int, candidates ...*Candidate) (Command, scheduling.Results, error) {
	if s.IsConsolidated() {
		return Command{}, scheduling.Results{}, nil
	}
	candidates = s.sortCandidates(candidates)

	v := NewValidation(s.clock, s.cluster, s.kubeClient, s.provisioner, s.cloudProvider, s.recorder, s.queue, v1beta1.DisruptionReasonUnderutilized)

	// Set a timeout
	timeout := s.clock.Now().Add(SingleNodeConsolidationTimeoutDuration)
	constrainedByBudgets := false
	// binary search to find the maximum number of NodeClaims we can terminate
	for i, candidate := range candidates {
		// Filter out empty candidates. If there was an empty node that wasn't consolidated before this, we should
		// assume that it was due to budgets. If we don't filter out budgets, users who set a budget for `empty`
		// can find their nodes disrupted here.
		if candidate.NodeClaim.StatusConditions().Get(v1beta1.ConditionTypeEmpty) != nil {
			continue
		}
		// If the disruption budget doesn't allow this candidate to be disrupted,
		// continue to the next candidate. We don't need to decrement any budget
		// counter since single node consolidation commands can only have one candidate.
		if disruptionBudgetMapping[candidate.nodePool.Name][v1beta1.DisruptionReasonUnderutilized] == 0 {
			constrainedByBudgets = true
			continue
		}
		if s.clock.Now().After(timeout) {
			ConsolidationTimeoutTotalCounter.WithLabelValues(s.ConsolidationType()).Inc()
			log.FromContext(ctx).V(1).Info(fmt.Sprintf("abandoning single-node consolidation due to timeout after evaluating %d candidates", i))
			return Command{}, scheduling.Results{}, nil
		}
		// compute a possible consolidation option
		cmd, results, err := s.computeConsolidation(ctx, candidate)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed computing consolidation")
			continue
		}
		if cmd.Action() == NoOpAction {
			continue
		}
		if err := v.IsValid(ctx, cmd, consolidationTTL); err != nil {
			if IsValidationError(err) {
				log.FromContext(ctx).V(1).Info(fmt.Sprintf("abandoning single-node consolidation attempt due to pod churn, command is no longer valid, %s", cmd))
				return Command{}, scheduling.Results{}, nil
			}
			return Command{}, scheduling.Results{}, fmt.Errorf("validating consolidation, %w", err)
		}
		return cmd, results, nil
	}
	if !constrainedByBudgets {
		// if there are no candidates because of a budget, don't mark
		// as consolidated, as it's possible it should be consolidatable
		// the next time we try to disrupt.
		s.markConsolidated()
	}
	return Command{}, scheduling.Results{}, nil
}

func (s *SingleNodeConsolidation) Type() string {
	return metrics.ConsolidationReason
}

func (s *SingleNodeConsolidation) ConsolidationType() string {
	return "single"
}
