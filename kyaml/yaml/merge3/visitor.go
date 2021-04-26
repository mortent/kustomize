// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package merge3

import (
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/kustomize/kyaml/yaml/walk"
)

type ConflictType int

const (
	UpdatedRemoved ConflictType = iota
	DestRemoved
	AddedInUpdatedAndLocal
)

type ConflictResolver interface {
	MapConflict(ConflictType, walk.Sources, *openapi.ResourceSchema) (*yaml.RNode, error)
	AListConflict(ConflictType, walk.Sources, *openapi.ResourceSchema) (*yaml.RNode, error)
	ScalarConflict(walk.Sources, *openapi.ResourceSchema) (*yaml.RNode, error)
	NAListConflict(walk.Sources, *openapi.ResourceSchema) (*yaml.RNode, error)
}

type Visitor struct{
	ConflictResolver ConflictResolver
}

func (m Visitor) VisitMap(nodes walk.Sources, s *openapi.ResourceSchema) (*yaml.RNode, error) {
	if !nodes.Origin().IsTaggedNull() && (nodes.Updated().IsTaggedNull() || nodes.Dest().IsTaggedNull()) {
		// explicitly cleared from either dest or update
		return walk.ClearNode, nil
	}

	originExists := yaml.Exists(nodes.Origin())
	updatedExists := yaml.Exists(nodes.Updated())
	destExists := yaml.Exists(nodes.Dest())

	switch {
	case originExists && updatedExists && destExists:
		// If found in dest, updated and origin, just merge recursively
		// with dest.
		return nodes.Dest(), nil
	case originExists && updatedExists && !destExists:
		// No longer exists in dest, delegate to ConflictResolver.
		return m.ConflictResolver.MapConflict(DestRemoved, nodes, s)
	case originExists && !updatedExists && destExists:
		// No longer exists in update, delegate to ConflictResolver.
		return m.ConflictResolver.MapConflict(UpdatedRemoved, nodes, s)
	case originExists && !updatedExists && !destExists:
		// If missing from both updated and dest, clear it.
		return walk.ClearNode, nil
	case !originExists && updatedExists && destExists:
		// Added in both update and destination. Delegate to ConflictResolver
		return m.ConflictResolver.MapConflict(AddedInUpdatedAndLocal, nodes, s)
	case !originExists && updatedExists && !destExists:
		// If only exists in update, just take it.
		return nodes.Updated(), nil
	case !originExists && !updatedExists && destExists:
		// If only exists in dest, just take it.
		return nodes.Dest(), nil
	default: // !originExists && !updatedExists && !destExists:
		// If the node doesn't exist in origin, updated or dest, we shouldn't
		// get here.
		return walk.ClearNode, nil
	}
}

func (m Visitor) visitAList(nodes walk.Sources, s *openapi.ResourceSchema) (*yaml.RNode, error) {
	originExists := yaml.Exists(nodes.Origin())
	updatedExists := yaml.Exists(nodes.Updated())
	destExists := yaml.Exists(nodes.Dest())

	switch {
	case originExists && updatedExists && destExists:
		// If found in dest, updated and origin, just merge recursively
		// with dest.
		return nodes.Dest(), nil
	case originExists && updatedExists && !destExists:
		// No longer exists in local. Delegate to ConflictResolver.
		return m.ConflictResolver.AListConflict(DestRemoved, nodes, s)
	case originExists && !updatedExists && destExists:
		// No longer exists in local. Delegate to ConflictResolver.
		return m.ConflictResolver.AListConflict(UpdatedRemoved, nodes, s)
	case originExists && !updatedExists && !destExists:
		// If missing from both updated and dest, clear it.
		return walk.ClearNode, nil
	case !originExists && updatedExists && destExists:
		// No longer exists in local. Delegate to ConflictResolver.
		return m.ConflictResolver.AListConflict(AddedInUpdatedAndLocal, nodes, s)
	case !originExists && updatedExists && !destExists:
		// If only exists in update, just take it.
		return nodes.Updated(), nil
	case !originExists && !updatedExists && destExists:
		// If only exists in dest, just take it.
		return nodes.Dest(), nil
	default: // !originExists && !updatedExists && !destExists:
		// If the node doesn't exist in origin, updated or dest, we shouldn't
		// get here.
		return walk.ClearNode, nil
	}
}

func (m Visitor) VisitScalar(nodes walk.Sources, s *openapi.ResourceSchema) (*yaml.RNode, error) {
	if !nodes.Origin().IsTaggedNull() && (nodes.Updated().IsTaggedNull() || nodes.Dest().IsTaggedNull()) {
		// explicitly cleared from either dest or update
		return walk.ClearNode, nil
	}

	values, err := m.getStrValues(nodes)
	if err != nil {
		return nil, err
	}

	destChanged := values.Dest != values.Origin
	updatedChanged := values.Update != values.Origin

	switch {
	case !destChanged && !updatedChanged:
		// Neither destination nor updated has changed, so we can take either value
		// since they must be the same.
		return nodes.Dest(), nil
	case !destChanged && updatedChanged:
		return nodes.Updated(), nil
	case destChanged && !updatedChanged:
		return nodes.Dest(), nil
	}

	return m.ConflictResolver.ScalarConflict(nodes, s)
}

func (m Visitor) visitNAList(nodes walk.Sources, s *openapi.ResourceSchema) (*yaml.RNode, error) {
	// compare origin and update values to see if they have changed
	values, err := m.getStrValues(nodes)
	if err != nil {
		return nil, err
	}

	destChanged := values.Dest != values.Origin
	updatedChanged := values.Update != values.Origin

	switch {
	case !destChanged && !updatedChanged:
		// Neither destination nor updated has changed, so we can take either value
		// since they must be the same.
		return nodes.Dest(), nil
	case !destChanged && updatedChanged:
		return nodes.Updated(), nil
	case destChanged && !updatedChanged:
		return nodes.Dest(), nil
	}

	return m.ConflictResolver.NAListConflict(nodes, s)
}

func (m Visitor) VisitList(nodes walk.Sources, s *openapi.ResourceSchema, kind walk.ListKind) (*yaml.RNode, error) {
	if !nodes.Origin().IsTaggedNull() && (nodes.Updated().IsTaggedNull() || nodes.Dest().IsTaggedNull()) {
		// explicitly cleared from either dest or update
		return walk.ClearNode, nil
	}

	if kind == walk.AssociativeList {
		return m.visitAList(nodes, s)
	}
	// non-associative list
	return m.visitNAList(nodes, s)
}

func (m Visitor) getStrValues(nodes walk.Sources) (strValues, error) {
	var uStr, oStr, dStr string
	var err error
	if nodes.Updated() != nil && nodes.Updated().YNode() != nil {
		s := nodes.Updated().YNode().Style
		defer func() {
			nodes.Updated().YNode().Style = s
		}()
		nodes.Updated().YNode().Style = yaml.FlowStyle | yaml.SingleQuotedStyle
		uStr, err = nodes.Updated().String()
		if err != nil {
			return strValues{}, err
		}
	}
	if nodes.Origin() != nil && nodes.Origin().YNode() != nil {
		s := nodes.Origin().YNode().Style
		defer func() {
			nodes.Origin().YNode().Style = s
		}()
		nodes.Origin().YNode().Style = yaml.FlowStyle | yaml.SingleQuotedStyle
		oStr, err = nodes.Origin().String()
		if err != nil {
			return strValues{}, err
		}
	}
	if nodes.Dest() != nil && nodes.Dest().YNode() != nil {
		s := nodes.Dest().YNode().Style
		defer func() {
			nodes.Dest().YNode().Style = s
		}()
		nodes.Dest().YNode().Style = yaml.FlowStyle | yaml.SingleQuotedStyle
		dStr, err = nodes.Dest().String()
		if err != nil {
			return strValues{}, err
		}
	}

	return strValues{Origin: oStr, Update: uStr, Dest: dStr}, nil
}

type strValues struct {
	Origin string
	Update string
	Dest   string
}

var _ walk.Visitor = Visitor{}
