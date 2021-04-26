// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package merge3_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	. "sigs.k8s.io/kustomize/kyaml/yaml/merge3"
	"sigs.k8s.io/kustomize/kyaml/yaml/merge3/resolvers"
)

var testCases = [][]testCase{scalarTestCases, listTestCases, mapTestCases, elementTestCases, kustomizationTestCases}

const (
	takeUpdateStrategy = "takeUpdate"
	takeLocalStrategy = "takeLocal"
)

var conflictResolvers = map[string]ConflictResolver{
	takeUpdateStrategy: &resolvers.TakeUpdateConflictResolver{},
	takeLocalStrategy: &resolvers.TakeLocalConflictResolver{},
}

func TestMerge(t *testing.T) {
	for i := range testCases {
		for j := range testCases[i] {
			tc := testCases[i][j]

			expectedForStrategy := make(map[string]string)
			if len(tc.expectedForStrategy) == 0 {
				expectedForStrategy[takeUpdateStrategy] = tc.expected
				expectedForStrategy[takeLocalStrategy] = tc.expected
			} else {
				expectedForStrategy = tc.expectedForStrategy
			}

			var strategies []string
			for s := range expectedForStrategy {
				strategies = append(strategies, s)
			}
			for _, strategy := range strategies {
				t.Run(fmt.Sprintf("%s-%s", tc.description, strategy), func(t *testing.T) {
					resolver := conflictResolvers[strategy]
					actual, err := MergeStrings(tc.local, tc.origin, tc.update, tc.infer, resolver)
					if tc.err == nil {
						if !assert.NoError(t, err, tc.description) {
							t.FailNow()
						}
						expected := expectedForStrategy[strategy]
						if !assert.Equal(t,
							strings.TrimSpace(expected), strings.TrimSpace(actual), tc.description) {
							t.FailNow()
						}
					} else {
						if !assert.Errorf(t, err, tc.description) {
							t.FailNow()
						}
						if !assert.Contains(t, tc.err.Error(), err.Error()) {
							t.FailNow()
						}
					}
				})
			}
		}
	}
}

type testCase struct {
	description string
	origin      string
	update      string
	local       string
	expected    string
	expectedForStrategy map[string]string
	err         error
	infer       bool
}
