package observers

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

type ResourceObserver interface {
	Observe(ctx context.Context, resource wait.ResourceIdentifier) *common.ObservedResource

	ObserveObject(ctx context.Context, deployment *unstructured.Unstructured) *common.ObservedResource
}
