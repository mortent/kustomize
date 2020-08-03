// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package setters2

import (
	"fmt"

	"github.com/go-openapi/spec"
	"sigs.k8s.io/kustomize/kyaml/fieldmeta"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// visitor is implemented by structs which need to walk the configuration.
// visitor is provided to accept to walk configuration
type visitor interface {
	// visitScalar is called for each scalar field value on a resource
	// node is the scalar field value
	// path is the path to the field; path elements are separated by '.'
	// oa is the OpenAPI schema for the field
	visitScalar(node *yaml.RNode, path string, oa, fieldOA *openapi.ResourceSchema) error

	// visitSequence is called for each sequence field value on a resource
	// node is the sequence field value
	// path is the path to the field
	// oa is the OpenAPI schema for the field
	visitSequence(node *yaml.RNode, path string, oa *openapi.ResourceSchema) error

	// visitMapping is called for each Mapping field value on a resource
	// node is the mapping field value
	// path is the path to the field
	// oa is the OpenAPI schema for the field
	visitMapping(node *yaml.RNode, path string, oa *openapi.ResourceSchema) error

	visitEmpty(parent *yaml.RNode, field, parentPath string, oa, fieldOA *openapi.ResourceSchema) error
}

// accept invokes the appropriate function on v for each field in object
func accept(v visitor, object *yaml.RNode) error {
	// get the OpenAPI for the type if it exists
	oa := getSchema(object, nil, "")
	return acceptImpl(v, object, "", oa)
}

// acceptImpl implements accept using recursion
func acceptImpl(v visitor, object *yaml.RNode, p string, oa *openapi.ResourceSchema) error {
	if object.YNode().Kind == yaml.MappingNode && !oa.IsEmpty() {
		for field := range oa.Schema.Properties {
			schema := oa.Schema.Properties[field]
			if schema.Ref.String() != "" {
				s, err := openapi.Resolve(&schema.Ref)
				if err == nil && s != nil {
					schema = *s
				}
			}

			node, err := object.Pipe(yaml.Lookup(field))
			if err != nil {
				return err
			}

			// If exists we do nothing. It will be visited during the
			// walk of the yaml doc.
			if node != nil {
				continue
			}
			// We only consider fields.
			if schema.Type.Contains("object") || schema.Type.Contains("array") {
				continue
			}

			oa := &openapi.ResourceSchema{
				Schema: &schema,
			}

			fieldOA := getSchema(nil, oa, field)

			if err := v.visitEmpty(object, field, p, oa, fieldOA); err != nil {
				return err
			}
		}
	}

	switch object.YNode().Kind {
	case yaml.DocumentNode:
		// Traverse the child of the document
		return accept(v, yaml.NewRNode(object.YNode()))
	case yaml.MappingNode:
		if err := v.visitMapping(object, p, oa); err != nil {
			return err
		}
		return object.VisitFields(func(node *yaml.MapNode) error {
			// get the schema for the field and propagate it
			fieldSchema := getSchema(node.Key, oa, node.Key.YNode().Value)
			// Traverse each field value
			return acceptImpl(v, node.Value, p+"."+node.Key.YNode().Value, fieldSchema)
		})
	case yaml.SequenceNode:
		// get the schema for the sequence node, use the schema provided if not present
		// on the field
		if err := v.visitSequence(object, p, oa); err != nil {
			return err
		}
		// get the schema for the elements
		schema := getSchema(object, oa, "")
		return object.VisitElements(func(node *yaml.RNode) error {
			// Traverse each list element
			return acceptImpl(v, node, p, schema)
		})
	case yaml.ScalarNode:
		// Visit the scalar field
		fieldSchema := getSchema(object, oa, "")
		return v.visitScalar(object, p, oa, fieldSchema)
	}
	return nil
}

// getSchema returns OpenAPI schema for an RNode or field of the
// RNode.  It will overriding the provide schema with field specific values
// if they are found
// r is the Node to get the Schema for
// s is the provided schema for the field if known
// field is the name of the field
func getSchema(r *yaml.RNode, s *openapi.ResourceSchema, field string) *openapi.ResourceSchema {
	ref, err := spec.NewRef(fmt.Sprintf("#/definitions/io.k8s.cli.setters.krm.%s", field))
	if err == nil {
		s, err := openapi.Resolve(&ref)
		if err == nil && s != nil {
			return &openapi.ResourceSchema{
				Schema: s,
			}
		}
	}
	if r == nil {
		return nil
	}

	// get the override schema if it exists on the field
	fm := fieldmeta.FieldMeta{}
	if err := fm.Read(r); err == nil && !fm.IsEmpty() {
		// per-field schema, this is fine
		if fm.Schema.Ref.String() != "" {
			// resolve the reference
			s, err := openapi.Resolve(&fm.Schema.Ref)
			if err == nil && s != nil {
				fm.Schema = *s
			}
		}
		return &openapi.ResourceSchema{Schema: &fm.Schema}
	}

	// get the schema for a field of the node if the field is provided
	if s != nil && field != "" {
		return s.Field(field)
	}

	// get the schema for the elements if this is a list
	if s != nil && r.YNode().Kind == yaml.SequenceNode {
		return s.Elements()
	}

	// use the provided schema if present
	if s != nil {
		return s
	}

	if yaml.IsEmpty(r) {
		return nil
	}

	// lookup the schema for the type
	m, _ := r.GetMeta()
	if m.Kind == "" || m.APIVersion == "" {
		return nil
	}
	return openapi.SchemaForResourceType(yaml.TypeMeta{Kind: m.Kind, APIVersion: m.APIVersion})
}
