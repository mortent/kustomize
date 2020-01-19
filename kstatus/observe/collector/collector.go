package collector

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"sigs.k8s.io/kustomize/kstatus/observe"
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

func NewObservedStatusCollector(identifiers []wait.ResourceIdentifier) *ObservedStatusCollector {
	observations := make(map[wait.ResourceIdentifier]*common.ObservedResource)
	for _, id := range identifiers {
		observations[id] = &common.ObservedResource{
			Identifier: id,
			Status: status.UnknownStatus,
		}
	}
	return &ObservedStatusCollector{
		aggregateStatus: status.UnknownStatus,
		observations: observations,
	}
}

type ObservedStatusCollector struct {
	mux sync.RWMutex

	aggregateStatus status.Status

	observations map[wait.ResourceIdentifier]*common.ObservedResource
}

func (o *ObservedStatusCollector) Observe(eventChannel <-chan observe.Event, stop <-chan struct{}) <-chan struct{} {
	completed := make(chan struct{})
	go func() {
		defer close(completed)
		for {
			select {
			case <-stop:
				return
			case event, more := <-eventChannel:
				if !more {
					return
				}
				o.processEvent(event)
			}
		}
	}()
	return completed
}

func (o *ObservedStatusCollector) processEvent(event observe.Event) {
	fmt.Fprintf(os.Stderr, "log event %s\n", event.AggregateStatus)
	o.mux.Lock()
	defer o.mux.Unlock()
	o.aggregateStatus = event.AggregateStatus
	if event.EventType == observe.ResourceUpdateEvent {
		observedResource := event.Resource
		o.observations[observedResource.Identifier] = observedResource
	}
}

type Observation struct {
	AggregateStatus status.Status

	ObservedResources []*common.ObservedResource
}

func (o *ObservedStatusCollector) LatestObservation() *Observation {
	o.mux.RLock()
	defer o.mux.RUnlock()

	var observedResources common.ObservedResources
	for _, observedResource := range o.observations {
		observedResources = append(observedResources, observedResource)
	}
	sort.Sort(observedResources)

	return &Observation{
		AggregateStatus: o.aggregateStatus,
		ObservedResources: observedResources,
	}
}
