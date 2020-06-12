// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-openapi/spec"
	y1_1 "gopkg.in/yaml.v2"
	y1_2 "gopkg.in/yaml.v3"
	"sigs.k8s.io/kustomize/kyaml/errors"
)

// typeToTag maps OpenAPI schema types to yaml 1.2 tags
var typeToTag = map[string]string{
	"string":  StringTag,
	"integer": IntTag,
	"boolean": BoolTag,
	"number":  "!!float",
}

// FormatNonStringStyle makes sure that values which parse as non-string values in yaml 1.1
// are correctly formatted given the Schema type.
func FormatNonStringStyle(node *Node, schema spec.Schema) {
	if len(schema.Type) != 1 {
		return
	}
	t := schema.Type[0]

	if !IsYaml1_1NonString(node) {
		return
	}
	switch {
	case t == "string" && schema.Format != "int-or-string":
		if (node.Style&DoubleQuotedStyle == 0) && (node.Style&SingleQuotedStyle == 0) {
			// must quote values so they are parsed as strings
			node.Style = DoubleQuotedStyle
		}
	case t == "boolean" || t == "integer" || t == "number":
		if (node.Style&DoubleQuotedStyle != 0) || (node.Style&SingleQuotedStyle != 0) {
			// must NOT quote the values so they aren't parsed as strings
			node.Style = 0
		}
	default:
		return
	}
	if tag, found := typeToTag[t]; found {
		// make sure the right tag is set
		node.Tag = tag
	}
}

// IsYaml1_1NonString returns true if the value parses as a non-string value in yaml 1.1
// when unquoted.
//
// Note: yaml 1.2 uses different keywords than yaml 1.1.  Example: yaml 1.2 interprets
// `field: on` and `field: "on"` as equivalent (both strings).  However Yaml 1.1 interprets
// `field: on` as on being a bool and `field: "on"` as on being a string.
// If an input is read with `field: "on"`, and the style is changed from DoubleQuote to 0,
// it will change the type of the field from a string  to a bool.  For this reason, fields
// which are keywords in yaml 1.1 should never have their style changed, as it would break
// backwards compatibility with yaml 1.1 -- which is what is used by the Kubernetes apiserver.
func IsYaml1_1NonString(node *Node) bool {
	if node.Kind != y1_2.ScalarNode {
		// not a keyword
		return false
	}
	return IsValueNonString(node.Value)
}

func IsValueNonString(value string) bool {
	if value == "" {
		return false
	}
	if strings.Contains(value, "\n") {
		// multi-line strings will fail to unmarshal
		return false
	}
	// check if the value will unmarshal into a non-string value using a yaml 1.1 parser
	var i1 interface{}
	if err := y1_1.Unmarshal([]byte(value), &i1); err != nil {
		return false
	}
	if reflect.TypeOf(i1) != stringType {
		return true
	}

	return false
}

var stringType = reflect.TypeOf("string")

// ValidValueForTag checks whether the given value is valid for the provided
// yaml tag.
func ValidValueForTag(value, tag string) (bool, error) {
	var regexps []string
	switch tag {
	case BoolTag:
		// Regexps from https://yaml.org/type/bool.html
		regexps = []string{
			`^(y|Y|yes|Yes|YES|n|N|no|No|NO|true|True|TRUE)$`,
			`^(false|False|FALSE|on|On|ON|off|Off|OFF)$`,
		}
	case FloatTag:
		// Regexps from https://yaml.org/type/float.html
		regexps = []string{
			`^([-+]?([0-9][0-9_]*)?\.[0-9._]*([eE][-+][0-9]+)?)$`,
			`^([-+]?[0-9][0-9_]*(:[0-5]?[0-9])+\.[0-9_]*)$`,
			`^([-+]?\.(inf|Inf|INF))$`,
			`^(\.(nan|NaN|NAN))$`,
		}
	case NullTag:
		// Regexps from https://yaml.org/type/null.html
		regexps = []string{`^(~|null|Null|NULL)$`}
	case IntTag:
		// Regexps from https://yaml.org/type/int.html
		regexps = []string{
			`^([-+]?0b[0-1_]+)$`,
			`^([-+]?0[0-7_]+)$`,
			`^([-+]?(0|[1-9][0-9_]*))$`,
			`^([-+]?0x[0-9a-fA-F_]+)$`,
			`^([-+]?[1-9][0-9_]*(:[0-5]?[0-9])+)$`,
		}
	case StringTag:
		return true, nil
	default:
		return false, fmt.Errorf("unknown tag value: %s", tag)
	}
	return matchesRegex(value, regexps)
}

// matchesRegex checks whether the given value matches any of the provided
// regexp expressions. If at least one matches, this function will return
// true.
func matchesRegex(val string, regexps []string) (bool, error) {
	for _, re := range regexps {
		match, err := regexp.MatchString(re, val)
		if err != nil {
			return false, errors.Wrap(err)
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}
