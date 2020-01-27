package observers

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/reader"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
	"sort"
)

func NewReplicaSetObserver(reader reader.ObserverReader, mapper meta.RESTMapper, podObserver *PodObserver) *ReplicaSetObserver {
	return &ReplicaSetObserver{
		BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
		PodObserver: podObserver,
	}
}

type ReplicaSetObserver struct {
	BaseObserver

	PodObserver *PodObserver
}

func (r *ReplicaSetObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	rs, observedResource := r.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return r.ObserveReplicaSet(ctx, rs)
}

func (r *ReplicaSetObserver) ObserveReplicaSet(ctx context.Context, rs *unstructured.Unstructured) *common.ObservedResource {
	identifier := r.ToIdentifier(rs)

	namespace := common.GetNamespaceForNamespacedResource(rs)
	selector, err := r.ToSelector(rs, "spec", "selector")
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
		observedPod := r.PodObserver.ObservePod(ctx, &pod)
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