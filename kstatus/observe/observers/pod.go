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

func NewPodObserver(reader reader.ObserverReader, mapper meta.RESTMapper) *PodObserver {
	return &PodObserver{
		BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
	}
}

type PodObserver struct {
	BaseObserver
}

func (r *PodObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	pod, observedResource := r.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return r.ObserveObject(ctx, pod)
}

func (r *PodObserver) ObserveObject(_ context.Context, pod *unstructured.Unstructured) *common.ObservedResource {
	identifier := toIdentifier(pod)

	res, err := status.Compute(pod)
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
		Resource: pod,
		Message: res.Message,
	}
}