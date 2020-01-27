package observers

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/reader"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func NewDefaultObserver(reader reader.ObserverReader, mapper meta.RESTMapper) *DefaultObserver {
	return &DefaultObserver{
		BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
	}
}

type DefaultObserver struct {
	BaseObserver
}

func (d *DefaultObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	u, observedResource := d.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return d.ObserveResource(ctx, u)
}

func (d *DefaultObserver) ObserveResource(_ context.Context, resource *unstructured.Unstructured) *common.ObservedResource {
	identifier := d.ToIdentifier(resource)

	res, err := status.Compute(resource)
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Error: err,
		}
	}

	return &common.ObservedResource{
		Identifier: identifier,
		Status: res.Status,
		Resource: resource,
		Message: res.Message,
	}
}