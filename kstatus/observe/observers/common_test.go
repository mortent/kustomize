package observers

import (
	"context"
	"fmt"
	"testing"

	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/kstatus/observe/testutil"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
	deploymentGVR = appsv1.SchemeGroupVersion.WithResource("deployments")
)

func TestLookupResource(t *testing.T) {
	deploymentIdentifier := wait.ResourceIdentifier{
		GroupKind: deploymentGVK.GroupKind(),
		Name: "Foo",
		Namespace: "Bar",
	}

	testCases := map[string]struct{
		identifier wait.ResourceIdentifier
		err error
		returnsObservedResource bool
		expectedStatus status.Status
	} {
		"unknown GVK": {
			identifier: wait.ResourceIdentifier{
				GroupKind: schema.GroupKind{
					Group: "custom.io",
					Kind: "Custom",
				},
				Name: "Bar",
				Namespace: "default",
			},
			returnsObservedResource: true,
			expectedStatus: status.UnknownStatus,
		},
		"resource does not exist": {
			identifier: deploymentIdentifier,
			err: errors.NewNotFound(deploymentGVR.GroupResource(), "Foo"),
			returnsObservedResource: true,
			expectedStatus: status.NotFoundStatus,
		},
		"getting resource fails": {
			identifier: deploymentIdentifier,
			err: errors.NewInternalError(fmt.Errorf("this is a test")),
			returnsObservedResource: true,
			expectedStatus: status.UnknownStatus,
		},
		"getting resource succeeds": {
			identifier: deploymentIdentifier,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			fakeReader := &fakeReader{
				getErr: tc.err,
			}
			fakeMapper := testutil.NewFakeRESTMapper(deploymentGVK)

			baseObserver := &BaseObserver{
				Reader: fakeReader,
				Mapper: fakeMapper,
			}

			u, observedResource := baseObserver.LookupResource(context.Background(), tc.identifier)

			if tc.returnsObservedResource {
				assert.Equal(t, tc.expectedStatus, observedResource.Status)
			} else {
				assert.Equal(t, deploymentGVK, u.GroupVersionKind())
			}
		})
	}
}
