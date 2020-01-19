package observe

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/observers"
	"sigs.k8s.io/kustomize/kstatus/status"
	"time"

	"sigs.k8s.io/kustomize/kstatus/wait"
)

type ResourceObserver interface {
	Observe(ctx context.Context, resource wait.ResourceIdentifier) *common.ObservedResource
}

type StatusAggregator interface {
	ResourceObserved(resource *common.ObservedResource)
	AggregateStatus() status.Status
	Completed() bool
}

func NewStatusObserver(reader client.Reader, mapper meta.RESTMapper) *StatusObserver {
	podObserver := observers.NewPodObserver(reader, mapper)
	replicaSetObserver := observers.NewReplicaSetObserver(reader, mapper, podObserver)
	deploymentObserver := observers.NewDeploymentObserver(reader, mapper, replicaSetObserver)
	statefulSetObserver := observers.NewStatefulSetObserver(reader, mapper, podObserver)
	jobObserver := observers.NewJobObserver(reader, mapper, podObserver)
	return &StatusObserver{
		observers: map[schema.GroupKind]ResourceObserver{
			appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind(): deploymentObserver,
			appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind(): statefulSetObserver,
			appsv1.SchemeGroupVersion.WithKind("ReplicaSet").GroupKind(): replicaSetObserver,
			v1.SchemeGroupVersion.WithKind("Pod").GroupKind(): podObserver,
			batchv1.SchemeGroupVersion.WithKind("Job").GroupKind(): jobObserver,
		},
		defaultObserver: observers.NewDefaultObserver(reader, mapper),
	}
}

type StatusObserver struct{
	observers map[schema.GroupKind]ResourceObserver
	defaultObserver ResourceObserver
}

type EventType string

const (
	ResourceUpdateEvent EventType = "ResourceUpdated"
	CompletedEvent EventType = "Completed"
	AbortedEvent EventType = "Aborted"
)

type Event struct {
	EventType EventType

	AggregateStatus status.Status

	Resource *common.ObservedResource
}

func (s *StatusObserver) Observe(ctx context.Context, resources []wait.ResourceIdentifier, stopOnCompleted bool) <-chan Event {
	eventChannel := make(chan Event)

	runner := StatusObserverRunner{
		ctx:                       ctx,
		identifiers:               resources,
		eventChannel:              eventChannel,
		observer:                  s,
		previousObservedResources: make(map[wait.ResourceIdentifier]*common.ObservedResource),
		statusAggregator:          NewAllCurrentOrNotFoundStatusAggregator(resources),
		stopOnCompleted:           stopOnCompleted,
	}
	go runner.Run()

	return eventChannel
}

func (s *StatusObserver) observerForGroupKind(gk schema.GroupKind) ResourceObserver {
	observer, ok := s.observers[gk]
	if !ok {
		return s.defaultObserver
	}
	return observer
}

type StatusObserverRunner struct {
	ctx context.Context

	observer *StatusObserver

	identifiers []wait.ResourceIdentifier

	previousObservedResources map[wait.ResourceIdentifier]*common.ObservedResource

	eventChannel chan Event

	statusAggregator StatusAggregator

	stopOnCompleted bool
}

func (r *StatusObserverRunner) Run() {
	ticker := time.NewTicker(2 * time.Second)
	defer func() {
		ticker.Stop()
		close(r.eventChannel)
	}()

	for {
		select {
		case <-r.ctx.Done():
			aggregatedStatus := r.statusAggregator.AggregateStatus()
			r.eventChannel <- Event{
				EventType:       AbortedEvent,
				AggregateStatus: aggregatedStatus,
			}
			return
		case <-ticker.C:
			completed := r.pollAllResources()
			if completed {
				aggregatedStatus := r.statusAggregator.AggregateStatus()
				r.eventChannel <- Event{
					EventType:       CompletedEvent,
					AggregateStatus: aggregatedStatus,
				}
				return
			}
		}
	}
}

func (r *StatusObserverRunner) pollAllResources() bool {
	for _, id := range r.identifiers {
		gk := id.GroupKind
		observer := r.observer.observerForGroupKind(gk)
		observedResource := observer.Observe(r.ctx, id)
		r.statusAggregator.ResourceObserved(observedResource)
		if r.isUpdatedObservedResource(observedResource) {
			aggregatedStatus := r.statusAggregator.AggregateStatus()
			r.eventChannel <- Event{
				EventType:       ResourceUpdateEvent,
				AggregateStatus: aggregatedStatus,
				Resource:        observedResource,
			}
			if r.statusAggregator.Completed() && r.stopOnCompleted {
				return true
			}
		}
	}
	return false
}

func (r *StatusObserverRunner) isUpdatedObservedResource(observedResource *common.ObservedResource) bool {
	oldObservedResource, found := r.previousObservedResources[observedResource.Identifier]
	if !found {
		return true
	}
	return DeepEqual(observedResource, oldObservedResource)
}

type AllCurrentOrNotFoundStatusAggregator struct {
	resourceCurrentStatus map[wait.ResourceIdentifier]status.Status
}

func NewAllCurrentOrNotFoundStatusAggregator(identifiers []wait.ResourceIdentifier) *AllCurrentOrNotFoundStatusAggregator {
	aggregator := &AllCurrentOrNotFoundStatusAggregator{
		resourceCurrentStatus: make(map[wait.ResourceIdentifier]status.Status),
	}
	for _, id := range identifiers {
		aggregator.resourceCurrentStatus[id] = status.UnknownStatus
	}
	return aggregator
}

func (d *AllCurrentOrNotFoundStatusAggregator) ResourceObserved(observedResource *common.ObservedResource) {
	d.resourceCurrentStatus[observedResource.Identifier] = observedResource.Status
}

func (d *AllCurrentOrNotFoundStatusAggregator) AggregateStatus() status.Status {
	allCurrentOrNotFound := true
	anyUnknown := false
	for _, latestStatus := range d.resourceCurrentStatus {
		if latestStatus == status.FailedStatus {
			return status.FailedStatus
		}
		if latestStatus == status.UnknownStatus {
			anyUnknown = true
		}
		if !(latestStatus == status.CurrentStatus || latestStatus == status.NotFoundStatus) {
			allCurrentOrNotFound = false
		}
	}
	if anyUnknown {
		return status.UnknownStatus
	}
	if allCurrentOrNotFound {
		return status.CurrentStatus
	}
	return status.InProgressStatus
}

func (d *AllCurrentOrNotFoundStatusAggregator) Completed() bool {
	return d.AggregateStatus() == status.CurrentStatus
}

func DeepEqual(or1, or2 *common.ObservedResource) bool {
	if or1.Identifier != or2.Identifier ||
		or1.Status != or2.Status ||
		or1.Error != or2.Error ||
		or1.ShortMessage != or2.ShortMessage {
		return false
	}
	if len(or1.GeneratedResources) != len(or2.GeneratedResources) {
		return false
	}

	for i := range or1.GeneratedResources {
		if !DeepEqual(or1.GeneratedResources[i], or2.GeneratedResources[i]) {
			return false
		}
	}
	return true
}