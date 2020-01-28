package observers

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/testutil"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

type fakeReader struct {
	testutil.NoopObserverReader

	getResource *unstructured.Unstructured
	getErr error

	listResources *unstructured.UnstructuredList
	listErr error
}

func (f *fakeReader) Get(_ context.Context, key client.ObjectKey, u *unstructured.Unstructured) error {
	if f.getResource != nil {
		u.Object = f.getResource.Object
	}
	return f.getErr
}

func (f *fakeReader) ListNamespaceScoped(_ context.Context, list *unstructured.UnstructuredList, ns string, selector labels.Selector) error {
	if f.listResources != nil {
		list.Items = f.listResources.Items
	}
	return f.listErr
}


type fakeResourceObserver struct {}

func (f *fakeResourceObserver) Observe(ctx context.Context, resource wait.ResourceIdentifier) *common.ObservedResource {
	return nil
}

func (f *fakeResourceObserver) ObserveObject(ctx context.Context, object *unstructured.Unstructured) *common.ObservedResource {
	identifier := toIdentifier(object)
	return &common.ObservedResource{
		Identifier: identifier,
	}
}