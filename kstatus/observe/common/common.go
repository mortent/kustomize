package common

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func GetIdentifierOrDie(object runtime.Object, gvk schema.GroupVersionKind) wait.ResourceIdentifier {
	acc, err := meta.Accessor(object)
	if err != nil {
		panic(err)
	}

	return wait.ResourceIdentifier{
		GroupKind: gvk.GroupKind(),
		Name: acc.GetName(),
		Namespace: acc.GetNamespace(),
	}
}

func GetNamespaceForNamespacedResource(object runtime.Object) string {
	acc, err := meta.Accessor(object)
	if err != nil {
		panic(err)
	}
	ns := acc.GetNamespace()
	if ns == "" {
		return "default"
	}
	return ns
}

func KeyForNamespacedResource(identifier wait.ResourceIdentifier) types.NamespacedName {
	namespace := "default"
	if identifier.Namespace != "" {
		namespace = identifier.Namespace
	}
	return types.NamespacedName{
		Name: identifier.Name,
		Namespace: namespace,
	}
}