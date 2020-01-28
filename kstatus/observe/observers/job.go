package observers

import (
	"context"
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/reader"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func NewJobObserver(reader reader.ObserverReader, mapper meta.RESTMapper, podObserver ResourceObserver) *JobObserver {
	return &JobObserver{
		BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
		PodObserver: podObserver,
	}
}

type JobObserver struct {
	BaseObserver

	PodObserver ResourceObserver
}

func (j *JobObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	job, observedResource := j.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return j.ObserveObject(ctx, job)
}

func (j *JobObserver) ObserveObject(ctx context.Context, job *unstructured.Unstructured) *common.ObservedResource {
	identifier := toIdentifier(job)

	namespace := common.GetNamespaceForNamespacedResource(job)
	selector, err := toSelector(job, "spec", "selector")
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: job,
			Error: err,
		}
	}

	var podList unstructured.UnstructuredList
	podList.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("Pod"))
	err = j.Reader.ListNamespaceScoped(ctx, &podList, namespace, selector)
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: job,
			Error: err,
		}
	}

	var observedPods common.ObservedResources
	for i := range podList.Items {
		pod := podList.Items[i]
		observedPod := j.PodObserver.ObserveObject(ctx, &pod)
		observedPods = append(observedPods, observedPod)
	}
	sort.Sort(observedPods)

	res, err := status.Compute(job)
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Error: err,
			GeneratedResources: observedPods,
		}
	}

	return &common.ObservedResource{
		Identifier: identifier,
		Status: res.Status,
		Resource: job,
		Message: res.Message,
		GeneratedResources: observedPods,
	}
}