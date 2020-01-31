package collector

import (
	appsv1 "k8s.io/api/apps/v1"
	"sort"
	"testing"
	"time"

	"gotest.tools/assert"
	"sigs.k8s.io/kustomize/kstatus/observe"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func TestCollectorStopsWhenEventChannelIsClosed(t *testing.T) {
	var identifiers []wait.ResourceIdentifier

	collector := NewObservedStatusCollector(identifiers)

	eventCh := make(chan observe.Event)
	stopCh := make(chan struct{})
	defer close(stopCh)

	completedCh := collector.Observe(eventCh, stopCh)

	timer := time.NewTimer(3 * time.Second)

	close(eventCh)
	select {
	case <-timer.C:
		t.Errorf("expected collector to close the completedCh, but it didn't")
	case <-completedCh:
		timer.Stop()
	}
}

func TestCollectorStopWhenStopChannelIsClosed(t *testing.T) {
	var identifiers []wait.ResourceIdentifier

	collector := NewObservedStatusCollector(identifiers)

	eventCh := make(chan observe.Event)
	defer close(eventCh)
	stopCh := make(chan struct{})

	completedCh := collector.Observe(eventCh, stopCh)

	timer := time.NewTimer(3 * time.Second)

	close(stopCh)
	select {
	case <-timer.C:
		t.Errorf("expected collector to close the completedCh, but it didn't")
	case <-completedCh:
		timer.Stop()
	}
}

var (
	deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
	statefulSetGVK = appsv1.SchemeGroupVersion.WithKind("StatefulSet")
	resourceIdentifiers = map[string]wait.ResourceIdentifier{
		"deployment": {
			GroupKind: deploymentGVK.GroupKind(),
			Name: "Foo",
			Namespace: "default",
		},
		"statefulSet": {
			GroupKind: statefulSetGVK.GroupKind(),
			Name: "Bar",
			Namespace: "default",
		},
	}
)

func TestCollectorEventProcessing(t *testing.T) {
	testCases := map[string]struct{
		identifiers []wait.ResourceIdentifier
		events []observe.Event
	} {
		"no resources and no events": {
		},
		"single resource and single event": {
			identifiers: []wait.ResourceIdentifier{
				resourceIdentifiers["deployment"],
			},
			events: []observe.Event{
				{
					EventType:       observe.ResourceUpdateEvent,
					AggregateStatus: status.CurrentStatus,
					Resource: &common.ObservedResource{
						Identifier: resourceIdentifiers["deployment"],
					},
				},
			},
		},
		"multiple resources and multiple events": {
			identifiers: []wait.ResourceIdentifier{
				resourceIdentifiers["deployment"],
				resourceIdentifiers["statefulSet"],
			},
			events: []observe.Event{
				{
					EventType:       observe.ResourceUpdateEvent,
					AggregateStatus: status.UnknownStatus,
					Resource: &common.ObservedResource{
						Identifier: resourceIdentifiers["deployment"],
					},
				},
				{
					EventType:       observe.ResourceUpdateEvent,
					AggregateStatus: status.InProgressStatus,
					Resource: &common.ObservedResource{
						Identifier: resourceIdentifiers["statefulSet"],
					},
				},
				{
					EventType:       observe.ResourceUpdateEvent,
					AggregateStatus: status.CurrentStatus,
					Resource: &common.ObservedResource{
						Identifier: resourceIdentifiers["deployment"],
					},
				},
				{
					EventType:       observe.ResourceUpdateEvent,
					AggregateStatus: status.InProgressStatus,
					Resource: &common.ObservedResource{
						Identifier: resourceIdentifiers["statefulSet"],
					},
				},
			},
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {

			collector := NewObservedStatusCollector(tc.identifiers)

			eventCh := make(chan observe.Event)
			defer close(eventCh)
			stopCh := make(chan struct{})

			collector.Observe(eventCh, stopCh)

			var latestEvent *observe.Event
			latestEventByIdentifier := make(map[wait.ResourceIdentifier]observe.Event)
			for _, event := range tc.events {
				if event.Resource != nil {
					latestEventByIdentifier[event.Resource.Identifier] = event
				}
				e := event
				latestEvent = &e
				eventCh <- event
			}
			// Give the collector some time to process the event.
			<-time.NewTimer(time.Second).C

			observation := collector.LatestObservation()

			var expectedObservation *Observation
			if latestEvent != nil {
				expectedObservation = &Observation{
					LastEventType: latestEvent.EventType,
					AggregateStatus: latestEvent.AggregateStatus,
				}
			} else {
				expectedObservation = &Observation{
					AggregateStatus: status.UnknownStatus,
				}
			}

			var observedResources common.ObservedResources
			for _, event := range latestEventByIdentifier {
				observedResources = append(observedResources, event.Resource)
			}
			sort.Sort(observedResources)
			expectedObservation.ObservedResources = observedResources

			assert.DeepEqual(t, expectedObservation, observation)
		})
	}



}