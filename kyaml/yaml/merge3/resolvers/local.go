// Copyright 2021 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package resolvers

import (
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/merge3"
	"sigs.k8s.io/kustomize/kyaml/yaml/walk"
)

type TakeLocalConflictResolver struct {}

func (tl *TakeLocalConflictResolver) MapConflict(conflictType merge3.ConflictType, nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	switch conflictType {
	case merge3.DestRemoved:
		return yaml.NewRNode(&yaml.Node{Kind: yaml.MappingNode}), nil
	case merge3.UpdatedRemoved:
		return nodes.Dest(), nil
	case merge3.AddedInUpdatedAndLocal:
		return nodes.Dest(), nil
	default:
		panic(fmt.Errorf("unknown conflict type %q", conflictType))
	}
}

func (tl *TakeLocalConflictResolver) AListConflict(conflictType merge3.ConflictType, nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	switch conflictType {
	case merge3.DestRemoved:
		return walk.ClearNode, nil
	case merge3.UpdatedRemoved:
		return nodes.Dest(), nil
	case merge3.AddedInUpdatedAndLocal:
		return nodes.Dest(), nil
	default:
		panic(fmt.Errorf("unknown conflict type %q", conflictType))
	}
}

func (tl *TakeLocalConflictResolver) ScalarConflict(nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	return nodes.Dest(), nil
}

func (tl *TakeLocalConflictResolver) NAListConflict(nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	return nodes.Dest(), nil
}
