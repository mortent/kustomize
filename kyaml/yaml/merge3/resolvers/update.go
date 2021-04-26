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

type TakeUpdateConflictResolver struct {}

func (tu *TakeUpdateConflictResolver) MapConflict(conflictType merge3.ConflictType, nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
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

func (tu *TakeUpdateConflictResolver) AListConflict(conflictType merge3.ConflictType, nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	switch conflictType {
	case merge3.DestRemoved:
		return yaml.NewRNode(&yaml.Node{Kind: yaml.SequenceNode}), nil
	case merge3.UpdatedRemoved:
		return walk.ClearNode, nil
	case merge3.AddedInUpdatedAndLocal:
		return nodes.Dest(), nil
	default:
		panic(fmt.Errorf("unknown conflict type %q", conflictType))
	}
}

func (tu *TakeUpdateConflictResolver) ScalarConflict(nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	return nodes.Updated(), nil
}

func (tu *TakeUpdateConflictResolver) NAListConflict(nodes walk.Sources, _ *openapi.ResourceSchema) (*yaml.RNode, error) {
	return nodes.Updated(), nil
}
