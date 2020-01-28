package observers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/reader"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func NewServiceObserver(reader reader.ObserverReader, mapper meta.RESTMapper) *ServiceObserver {
	return &ServiceObserver{BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
	}
}

type ServiceObserver struct {
	BaseObserver

	GK schema.GroupKind
}

func (s *ServiceObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	service, observedResource := s.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return s.ObserveObject(ctx, service)
}

func (s *ServiceObserver) ObserveObject(_ context.Context, service *unstructured.Unstructured) *common.ObservedResource {
	identifier := toIdentifier(service)

	res, err := status.Compute(service)
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
		Resource: service,
		Message: res.Message,
	}
}