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

package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
)

// EvaluateResourceClaimSelector evaluates a CEL expression from a ResourceClaim against ResourceSlice attributes
func EvaluateResourceClaimSelector(expression string, attributes map[string]string) (bool, error) {
	// Create CEL environment with attribute declarations
	env, err := cel.NewEnv(
		cel.Declarations(
			decls.NewVar("attributes", decls.NewMapType(decls.String, decls.String)),
		),
	)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Parse and check the expression
	ast, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("failed to parse CEL expression: %w", issues.Err())
	}

	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("failed to check CEL expression: %w", issues.Err())
	}

	// Create the program
	program, err := env.Program(checked)
	if err != nil {
		return false, fmt.Errorf("failed to create CEL program: %w", err)
	}

	// Evaluate the expression
	out, _, err := program.Eval(map[string]interface{}{
		"attributes": attributes,
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL expression: %w", err)
	}

	// Convert result to bool
	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression did not evaluate to boolean")
	}

	return result, nil
}
