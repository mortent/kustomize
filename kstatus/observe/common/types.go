package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

type ObservedResource struct {

	Identifier wait.ResourceIdentifier

	Status status.Status

	Resource *unstructured.Unstructured

	Error error

	Message string

	GeneratedResources ObservedResources
}

type ObservedResources []*ObservedResource

func (g ObservedResources) Len() int {
	return len(g)
}

func (g ObservedResources) Less(i, j int) bool {
	idI := g[i].Identifier
	idJ := g[j].Identifier

	if idI.Namespace != idJ.Namespace {
		return idI.Namespace < idJ.Namespace
	}
	if idI.GroupKind.Group != idJ.GroupKind.Group {
		return idI.GroupKind.Group < idJ.GroupKind.Group
	}
	if idI.GroupKind.Kind != idJ.GroupKind.Kind {
		return idI.GroupKind.Kind < idJ.GroupKind.Kind
	}
	return idI.Name < idJ.Name
}

func (g ObservedResources) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}
