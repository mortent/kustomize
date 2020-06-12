// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package yaml_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestIsYaml1_1NonString(t *testing.T) {
	type testCase struct {
		val      string
		expected bool
	}

	testCases := []testCase{
		{val: "hello world", expected: false},
		{val: "2.0", expected: true},
		{val: "2", expected: true},
		{val: "true", expected: true},
		{val: "1.0\nhello", expected: false}, // multiline strings should always be false
		{val: "", expected: false},           // empty string should be considered a string
	}

	for k := range valueToTagMap {
		testCases = append(testCases, testCase{val: k, expected: true})
	}

	for _, test := range testCases {
		assert.Equal(t, test.expected,
			yaml.IsYaml1_1NonString(&yaml.Node{Kind: yaml.ScalarNode, Value: test.val}), test.val)
	}
}

func TestFormatNonStringStyle(t *testing.T) {
	n := yaml.MustParse(`apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: foo
        args:
        - bar
        - on
        image: nginx:1.7.9
        ports:
        - name: http
          containerPort: "80"
`)
	s := openapi.SchemaForResourceType(
		yaml.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"})

	args, err := n.Pipe(yaml.Lookup(
		"spec", "template", "spec", "containers", "[name=foo]", "args"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if !assert.NotNil(t, args) {
		t.FailNow()
	}
	on := args.YNode().Content[1]
	onS := s.Lookup(
		"spec", "template", "spec", "containers", openapi.Elements, "args", openapi.Elements)
	yaml.FormatNonStringStyle(on, *onS.Schema)

	containerPort, err := n.Pipe(yaml.Lookup(
		"spec", "template", "spec", "containers", "[name=foo]", "ports",
		"[name=http]", "containerPort"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	if !assert.NotNil(t, containerPort) {
		t.FailNow()
	}
	cpS := s.Lookup("spec", "template", "spec", "containers", openapi.Elements,
		"ports", openapi.Elements, "containerPort")
	if !assert.NotNil(t, cpS) {
		t.FailNow()
	}
	yaml.FormatNonStringStyle(containerPort.YNode(), *cpS.Schema)

	actual := n.MustString()
	expected := `apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
      - name: foo
        args:
        - bar
        - "on"
        image: nginx:1.7.9
        ports:
        - name: http
          containerPort: 80
`
	assert.Equal(t, expected, actual)
}

func TestValidValueForTag(t *testing.T) {
	availableTags := []string{
		yaml.BoolTag,
		yaml.FloatTag,
		yaml.NullTag,
		yaml.IntTag,
		yaml.StringTag,
	}

	testCases := []struct {
		value    string
		validTag string
	}{
		{
			value:    "Y",
			validTag: yaml.BoolTag,
		},
		{
			value:    "NO",
			validTag: yaml.BoolTag,
		},
		{
			value:    "True",
			validTag: yaml.BoolTag,
		},
		{
			value:    "on",
			validTag: yaml.BoolTag,
		},
		{
			value:    "6.8523015e+5",
			validTag: yaml.FloatTag,
		},
		{
			value:    "685.230_15e+03",
			validTag: yaml.FloatTag,
		},
		{
			value:    "685_230.15",
			validTag: yaml.FloatTag,
		},
		{
			value:    "190:20:30.15",
			validTag: yaml.FloatTag,
		},
		{
			value:    "-.inf",
			validTag: yaml.FloatTag,
		},
		{
			value:    ".NaN",
			validTag: yaml.FloatTag,
		},
		{
			value:    "~",
			validTag: yaml.NullTag,
		},
		{
			value:    "null",
			validTag: yaml.NullTag,
		},
		{
			value:    "685230",
			validTag: yaml.IntTag,
		},
		{
			value:    "+685_230",
			validTag: yaml.IntTag,
		},
		{
			value:    "02472256",
			validTag: yaml.IntTag,
		},
		{
			value:    "0x_0A_74_AE",
			validTag: yaml.IntTag,
		},
		{
			value:    "0b1010_0111_0100_1010_1110",
			validTag: yaml.IntTag,
		},
		{
			value:    "190:20:30",
			validTag: yaml.IntTag,
		},
		{
			value:    "yaml",
			validTag: yaml.StringTag,
		},
		{
			value:    "12abc",
			validTag: yaml.StringTag,
		},
	}

	for i := range testCases {
		test := testCases[i]
		for i := range availableTags {
			tag := availableTags[i]
			name := fmt.Sprintf("value: %s, tag: %s", test.value, tag)
			t.Run(name, func(t *testing.T) {
				isValid, err := yaml.ValidValueForTag(test.value, tag)
				if !assert.NoError(t, err) {
					t.FailNow()
				}
				// All values are valid strings.
				if tag == yaml.StringTag {
					assert.True(t, isValid)
					return
				}
				if test.validTag == tag {
					assert.True(t, isValid)
				} else {
					assert.False(t, isValid)
				}
			})
		}
	}
}

// valueToTagMap is a map of values interpreted as non-strings in yaml 1.1 when left
// unquoted.
// To keep compatibility with the yaml parser used by Kubernetes (yaml 1.1) make sure the values
// which are treated as non-string values are kept as non-string values.
// https://github.com/go-yaml/yaml/blob/v2/resolve.go
var valueToTagMap = func() map[string]string {
	val := map[string]string{}

	// https://yaml.org/type/null.html
	values := []string{"~", "null", "Null", "NULL"}
	for i := range values {
		val[values[i]] = yaml.NullTag
	}

	// https://yaml.org/type/bool.html
	values = []string{
		"y", "Y", "yes", "Yes", "YES", "true", "True", "TRUE", "on", "On", "ON", "n", "N", "no",
		"No", "NO", "false", "False", "FALSE", "off", "Off", "OFF"}
	for i := range values {
		val[values[i]] = yaml.BoolTag
	}

	// https://yaml.org/type/float.html
	values = []string{
		".nan", ".NaN", ".NAN", ".inf", ".Inf", ".INF",
		"+.inf", "+.Inf", "+.INF", "-.inf", "-.Inf", "-.INF"}
	for i := range values {
		val[values[i]] = yaml.FloatTag
	}

	return val
}()
