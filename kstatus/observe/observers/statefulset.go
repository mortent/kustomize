package observers

import (
	"context"
	"sigs.k8s.io/kustomize/kstatus/observe/reader"
	"sort"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func NewStatefulSetObserver(reader reader.ObserverReader, mapper meta.RESTMapper, podObserver *PodObserver) *StatefulSetObserver {
	return &StatefulSetObserver{
		BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
		PodObserver: podObserver,
	}
}

type StatefulSetObserver struct {
	BaseObserver

	PodObserver *PodObserver
}

func (s *StatefulSetObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	statefulSet, observedResource := s.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return s.ObserveStatefulSet(ctx, statefulSet)
}

func (s *StatefulSetObserver) ObserveStatefulSet(ctx context.Context, statefulSet *unstructured.Unstructured) *common.ObservedResource {
	identifier := s.ToIdentifier(statefulSet)

	namespace := common.GetNamespaceForNamespacedResource(statefulSet)
	selector, err := s.ToSelector(statefulSet, "spec", "selector")
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: statefulSet,
			Error: err,
		}
	}

	var podList unstructured.UnstructuredList
	podList.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("Pod"))
	err = s.Reader.ListNamespaceScoped(ctx, &podList, namespace, selector)
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: statefulSet,
			Error: err,
		}
	}

	var observedReplicaSets common.ObservedResources
	for i := range podList.Items {
		pod := podList.Items[i]
		observedReplicaSet := s.PodObserver.ObservePod(ctx, &pod)
		observedReplicaSets = append(observedReplicaSets, observedReplicaSet)
	}
	sort.Sort(observedReplicaSets)

	res, err := status.Compute(statefulSet)
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Error: err,
			GeneratedResources: observedReplicaSets,
		}
	}

	return &common.ObservedResource{
		Identifier: identifier,
		Status: res.Status,
		Resource: statefulSet,
		Message: res.Message,
		GeneratedResources: observedReplicaSets,
	}
}