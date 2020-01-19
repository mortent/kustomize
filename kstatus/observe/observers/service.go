package observers

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func NewServiceObserver(reader client.Reader, mapper meta.RESTMapper) *ServiceObserver {
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
	return s.ObserveService(ctx, service)
}

func (s *ServiceObserver) ObserveService(_ context.Context, service *unstructured.Unstructured) *common.ObservedResource {
	identifier := s.ToIdentifier(service)

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
		LongMessage: res.Message,
	}
}