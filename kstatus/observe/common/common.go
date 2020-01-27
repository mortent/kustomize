package common

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

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