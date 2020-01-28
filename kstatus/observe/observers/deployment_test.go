package observers

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/testutil"
	"sigs.k8s.io/kustomize/kstatus/status"
)

var (
	deploymentManifest = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: Foo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
`

	deploymentManifestInvalidSelector = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: Foo
spec:
  replicas: 1
`

	replicaSetManifest1 = `
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: Foo-12345
spec:
  replicas: 1
`

	replicaSetManifest2 = `
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: Foo-54321
spec:
  replicas: 14
`
)

func TestDeploymentObserver(t *testing.T) {
	testCases := map[string]struct{
		deploymentManifest string
		replicaSets []unstructured.Unstructured
		listErr error
		replicaSetStatuses []status.Status
		deploymentStatusResult *status.Result
		expectedStatus status.Status
	} {
		"invalid selector": {
			deploymentManifest: deploymentManifestInvalidSelector,
			expectedStatus: status.UnknownStatus,
		},
		"error listing replicasets": {
			deploymentManifest: deploymentManifest,
			listErr: fmt.Errorf("this is a test"),
			expectedStatus: status.UnknownStatus,
		},
		"no replicasets": {
			deploymentManifest: deploymentManifest,
			replicaSets: []unstructured.Unstructured{},
			deploymentStatusResult: &status.Result{
				Status: status.InProgressStatus,
			},
			expectedStatus: status.InProgressStatus,
		},
		"deployment has replicasets": {
			deploymentManifest: deploymentManifest,
			replicaSets: []unstructured.Unstructured{
				*testutil.YamlToUnstructured(t, replicaSetManifest1),
				*testutil.YamlToUnstructured(t, replicaSetManifest2),
			},
			deploymentStatusResult: &status.Result{
				Status: status.CurrentStatus,
			},
			expectedStatus: status.CurrentStatus,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			fakeReader := &fakeReader{
				listResources: &unstructured.UnstructuredList{
					Items: tc.replicaSets,
				},
				listErr: tc.listErr,
			}
			fakeMapper := testutil.NewFakeRESTMapper()
			fakeReplicaSetObserver := &fakeResourceObserver{}

			dep := testutil.YamlToUnstructured(t, tc.deploymentManifest)

			observer := NewDeploymentObserver(fakeReader, fakeMapper, fakeReplicaSetObserver)
			observer.computeStatusFunc = func(u *unstructured.Unstructured) (*status.Result, error) {
				return tc.deploymentStatusResult, nil
			}

			observedResource := observer.ObserveObject(context.Background(), dep)

			assert.Equal(t, tc.expectedStatus, observedResource.Status)
			assert.Equal(t, len(tc.replicaSets), len(observedResource.GeneratedResources))
			assert.Assert(t, sort.IsSorted(observedResource.GeneratedResources))
		})
	}
}

