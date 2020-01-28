package observers

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kstatus/observe/testutil"
)

func TestJobObserver(t *testing.T) {
	testCases := map[string]struct{
		jobManifest string
		pods []unstructured.Unstructured
		listPodsErr error
	} {

	}

	for tn, _ := range testCases {
		t.Run(tn, func(t *testing.T) {
			fakeReader := &fakeReader{
				listResources: &unstructured.UnstructuredList{
					Items: tc.pods,
				},
				listErr: tc.listPodsErr,
			}
			fakeMapper := testutil.NewFakeRESTMapper()
			fakePodObserver := &fakePodObserver{}
		})
	}
}
