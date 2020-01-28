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

func NewReplicaSetObserver(reader reader.ObserverReader, mapper meta.RESTMapper, podObserver ResourceObserver) *replicaSetObserver {
	return &replicaSetObserver{
		BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
		PodObserver: podObserver,
	}
}

type replicaSetObserver struct {
	BaseObserver

	PodObserver ResourceObserver
}

func (r *replicaSetObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	rs, observedResource := r.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return r.ObserveObject(ctx, rs)
}

func (r *replicaSetObserver) ObserveObject(ctx context.Context, rs *unstructured.Unstructured) *common.ObservedResource {
	identifier := toIdentifier(rs)

	namespace := common.GetNamespaceForNamespacedResource(rs)
	selector, err := toSelector(rs, "spec", "selector")
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: rs,
			Error: err,
		}
	}

	var podList unstructured.UnstructuredList
	podList.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("Pod"))
	err = r.Reader.ListNamespaceScoped(ctx, &podList, namespace, selector)
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: rs,
			Error: err,
		}
	}

	var observedPods common.ObservedResources
	for i := range podList.Items {
		pod := podList.Items[i]
		observedPod := r.PodObserver.ObserveObject(ctx, &pod)
		observedPods = append(observedPods, observedPod)
	}
	sort.Sort(observedPods)

	res, err := status.Compute(rs)
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
		Resource: rs,
		Message: res.Message,
		GeneratedResources: observedPods,
	}
}