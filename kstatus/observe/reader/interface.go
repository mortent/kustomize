package reader

import (
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObserverReader interface {
	Get(ctx context.Context, key client.ObjectKey, obj *unstructured.Unstructured) error
	ListNamespaceScoped(ctx context.Context, list *unstructured.UnstructuredList, namespace string, selector labels.Selector) error
	ListClusterScoped(ctx context.Context, list *unstructured.UnstructuredList, selector labels.Selector) error
	Sync(ctx context.Context) error
}