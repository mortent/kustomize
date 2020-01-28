package observe

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kstatus/observe/aggregator"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/observe/observers"
	"sigs.k8s.io/kustomize/kstatus/observe/reader"
	"sigs.k8s.io/kustomize/kstatus/status"
	"time"

	"sigs.k8s.io/kustomize/kstatus/wait"
)

type StatusAggregator interface {
	ResourceObserved(resource *common.ObservedResource)
	AggregateStatus() status.Status
	Completed() bool
}

func NewStatusObserver(reader client.Reader, mapper meta.RESTMapper) *StatusObserver {
	return &StatusObserver{
		reader: reader,
		mapper: mapper,
	}
}

type StatusObserver struct {
	reader client.Reader
	mapper meta.RESTMapper
}

type EventType string

const (
	ResourceUpdateEvent EventType = "ResourceUpdated"
	CompletedEvent      EventType = "Completed"
	AbortedEvent        EventType = "Aborted"
	ErrorEvent EventType = "Error"
)

type Event struct {
	EventType EventType

	AggregateStatus status.Status

	Resource *common.ObservedResource

	Error error
}

func (s *StatusObserver) Observe(ctx context.Context, resources []wait.ResourceIdentifier, stopOnCompleted bool, useCache bool) <-chan Event {
	eventChannel := make(chan Event)

	var observerReader reader.ObserverReader
	if useCache {
		var err error
		observerReader, err = reader.NewCachedObserverReader(s.reader, s.mapper, resources)
		if err != nil {
			panic(err)
		}
	} else {
		observerReader = &reader.NoCacheObserverReader{
			Reader: s.reader,
		}
	}

	runner := newStatusObserverRunner(ctx, observerReader, s.mapper, resources, eventChannel, stopOnCompleted)
	go runner.Run()

	return eventChannel
}

func newStatusObserverRunner(ctx context.Context, reader reader.ObserverReader, mapper meta.RESTMapper, identifiers []wait.ResourceIdentifier,
	eventChannel chan Event, stopOnCompleted bool) *StatusObserverRunner {
	resourceObservers, defaultObserver := createObservers(reader, mapper)
	return &StatusObserverRunner{
		ctx:                       ctx,
		reader:                    reader,
		identifiers:               identifiers,
		observers:                 resourceObservers,
		defaultObserver:           defaultObserver,
		eventChannel:              eventChannel,
		previousObservedResources: make(map[wait.ResourceIdentifier]*common.ObservedResource),
		statusAggregator:          aggregator.NewAllCurrentOrNotFoundStatusAggregator(identifiers),
		stopOnCompleted:           stopOnCompleted,
	}
}

type StatusObserverRunner struct {
	ctx context.Context

	reader reader.ObserverReader

	observers map[schema.GroupKind]observers.ResourceObserver

	defaultObserver observers.ResourceObserver

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
			err := r.reader.Sync(r.ctx)
			if err != nil {
				r.eventChannel <- Event{
					EventType: ErrorEvent,
					Error: err,
				}
				return
			}
			completed := r.observeStatusForAllResources()
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

func (r *StatusObserverRunner) observeStatusForAllResources() bool {
	for _, id := range r.identifiers {
		gk := id.GroupKind
		observer := r.observerForGroupKind(gk)
		observedResource := observer.Observe(r.ctx, id)
		r.statusAggregator.ResourceObserved(observedResource)
		if r.isUpdatedObservedResource(observedResource) {
			r.previousObservedResources[id] = observedResource
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
	if r.statusAggregator.Completed() && r.stopOnCompleted {
		return true
	}
	return false
}

func (r *StatusObserverRunner) observerForGroupKind(gk schema.GroupKind) observers.ResourceObserver {
	observer, ok := r.observers[gk]
	if !ok {
		return r.defaultObserver
	}
	return observer
}

func (r *StatusObserverRunner) isUpdatedObservedResource(observedResource *common.ObservedResource) bool {
	oldObservedResource, found := r.previousObservedResources[observedResource.Identifier]
	if !found {
		return true
	}
	return !DeepEqual(observedResource, oldObservedResource)
}

func createObservers(reader reader.ObserverReader, mapper meta.RESTMapper) (map[schema.GroupKind]observers.ResourceObserver, observers.ResourceObserver) {
	podObserver := observers.NewPodObserver(reader, mapper)
	replicaSetObserver := observers.NewReplicaSetObserver(reader, mapper, podObserver)
	deploymentObserver := observers.NewDeploymentObserver(reader, mapper, replicaSetObserver)
	statefulSetObserver := observers.NewStatefulSetObserver(reader, mapper, podObserver)
	jobObserver := observers.NewJobObserver(reader, mapper, podObserver)
	serviceObserver := observers.NewServiceObserver(reader, mapper)

	resourceObservers := map[schema.GroupKind]observers.ResourceObserver{
		appsv1.SchemeGroupVersion.WithKind("Deployment").GroupKind():  deploymentObserver,
		appsv1.SchemeGroupVersion.WithKind("StatefulSet").GroupKind(): statefulSetObserver,
		appsv1.SchemeGroupVersion.WithKind("ReplicaSet").GroupKind():  replicaSetObserver,
		v1.SchemeGroupVersion.WithKind("Pod").GroupKind():             podObserver,
		batchv1.SchemeGroupVersion.WithKind("Job").GroupKind():        jobObserver,
		v1.SchemeGroupVersion.WithKind("Service").GroupKind(): 				 serviceObserver,
	}
	defaultObserver := observers.NewDefaultObserver(reader, mapper)
	return resourceObservers, defaultObserver
}

func DeepEqual(or1, or2 *common.ObservedResource) bool {
	if or1.Identifier != or2.Identifier ||
		or1.Status != or2.Status ||
		or1.Message != or2.Message {
		return false
	}

	if or1.Error != nil && or2.Error != nil && or1.Error.Error() != or2.Error.Error() {
		return false
	}
	if (or1.Error == nil && or2.Error != nil) || (or1.Error != nil && or2.Error == nil)  {
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
