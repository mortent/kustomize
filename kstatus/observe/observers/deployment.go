package observers

import (
	"context"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/reader"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func NewDeploymentObserver(reader reader.ObserverReader, mapper meta.RESTMapper, rsObserver ResourceObserver) *DeploymentObserver {
	return &DeploymentObserver{
		BaseObserver: BaseObserver{
			Reader: reader,
			Mapper: mapper,
		},
		RsObserver: rsObserver,

		computeStatusFunc: status.Compute,
	}
}

type DeploymentObserver struct {
	BaseObserver

	RsObserver ResourceObserver

	computeStatusFunc computeStatusFunc
}

func (d *DeploymentObserver) Observe(ctx context.Context, identifier wait.ResourceIdentifier) *common.ObservedResource {
	deployment, observedResource := d.LookupResource(ctx, identifier)
	if observedResource != nil {
		return observedResource
	}
	return d.ObserveObject(ctx, deployment)
}

func (d *DeploymentObserver) ObserveObject(ctx context.Context, deployment *unstructured.Unstructured) *common.ObservedResource {
	identifier := toIdentifier(deployment)

	namespace := common.GetNamespaceForNamespacedResource(deployment)
	selector, err := toSelector(deployment, "spec", "selector")
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: deployment,
			Error: err,
		}
	}

	var rsList unstructured.UnstructuredList
	rsList.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("ReplicaSet"))
	err = d.Reader.ListNamespaceScoped(ctx, &rsList, namespace, selector)
	if err != nil {
		return &common.ObservedResource{
			Identifier: identifier,
			Status: status.UnknownStatus,
			Resource: deployment,
			Error: err,
		}
	}

	var observedReplicaSets common.ObservedResources
	for i := range rsList.Items {
		rs := rsList.Items[i]
		observedReplicaSet := d.RsObserver.ObserveObject(ctx, &rs)
		observedReplicaSets = append(observedReplicaSets, observedReplicaSet)
	}
	sort.Sort(observedReplicaSets)

	res, err := d.computeStatusFunc(deployment)
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
		Resource: deployment,
		Message: res.Message,
		GeneratedResources: observedReplicaSets,
	}
}
