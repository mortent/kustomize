package observers

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

type BaseObserver struct {
	Reader client.Reader

	Mapper meta.RESTMapper
}

func (b *BaseObserver) LookupResource(ctx context.Context, identifier wait.ResourceIdentifier) (*unstructured.Unstructured, *common.ObservedResource) {
	GVK, err := b.GVK(identifier.GroupKind)
	if err != nil {
		return nil, &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Error: err,
		}
	}

	var u unstructured.Unstructured
	u.SetGroupVersionKind(GVK)
	key := common.KeyForNamespacedResource(identifier)
	err = b.Reader.Get(ctx, key, &u)
	if err != nil && errors.IsNotFound(err) {
		return nil, &common.ObservedResource{
			Identifier: identifier,
			Status: status.NotFoundStatus,
			ShortMessage: "NotFound",
			LongMessage: "Resource doesn't exist",
		}
	}
	if err != nil {
		return nil, &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Error: err,
		}
	}
	u.SetNamespace(identifier.Namespace)
	return &u, nil
}

func (b *BaseObserver) GVK(gk schema.GroupKind) (schema.GroupVersionKind, error) {
	mapping, err := b.Mapper.RESTMapping(gk)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return mapping.GroupVersionKind, nil
}

func (b *BaseObserver) ToIdentifier(u *unstructured.Unstructured) wait.ResourceIdentifier {
	return wait.ResourceIdentifier{
		GroupKind: u.GroupVersionKind().GroupKind(),
		Name: u.GetName(),
		Namespace: u.GetNamespace(),
	}
}

func (b *BaseObserver) ToSelector(resource *unstructured.Unstructured, path ...string) (labels.Selector, error) {
	selector, found, err := unstructured.NestedMap(resource.Object, path...)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no selector found")
	}
	bytes, err := json.Marshal(selector)
	if err != nil {
		return nil, err
	}
	var s metav1.LabelSelector
	err = json.Unmarshal(bytes, &s)
	if err != nil {
		return nil, err
	}
	return metav1.LabelSelectorAsSelector(&s)
}
