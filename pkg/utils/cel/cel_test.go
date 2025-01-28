package cel

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCELEvaluation(t *testing.T) {
	RegisterFailHandler(Fail)
	g := NewWithT(t)

	tests := []struct {
		name       string
		expression string
		attributes map[string]string
		expected   bool
		expectErr  bool
	}{
		{
			name:       "simple equality match",
			expression: "attributes.vendor == 'vendor-a'",
			attributes: map[string]string{
				"vendor": "vendor-a",
				"model":  "t4",
			},
			expected:  true,
			expectErr: false,
		},
		{
			name:       "simple equality non-match",
			expression: "attributes.vendor == 'vendor-b'",
			attributes: map[string]string{
				"vendor": "vendor-a",
				"model":  "t4",
			},
			expected:  false,
			expectErr: false,
		},
		{
			name:       "multiple conditions match",
			expression: "attributes.vendor == 'vendor-a' && attributes.model == 't4'",
			attributes: map[string]string{
				"vendor": "vendor-a",
				"model":  "t4",
			},
			expected:  true,
			expectErr: false,
		},
		{
			name:       "multiple conditions non-match",
			expression: "attributes.vendor == 'vendor-a' && attributes.model == 'a100'",
			attributes: map[string]string{
				"vendor": "vendor-a",
				"model":  "t4",
			},
			expected:  false,
			expectErr: false,
		},
		{
			name:       "missing attribute",
			expression: "attributes.non_existent == 'value'",
			attributes: map[string]string{
				"vendor": "vendor-a",
			},
			expected:  false,
			expectErr: true,
		},
		{
			name:       "invalid expression",
			expression: "invalid expression",
			attributes: map[string]string{
				"vendor": "vendor-a",
			},
			expected:  false,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EvaluateResourceClaimSelector(tc.expression, tc.attributes)
			if tc.expectErr {
				g.Expect(err).ToNot(BeNil())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}
