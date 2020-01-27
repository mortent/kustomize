package observe

import (
	"sigs.k8s.io/kustomize/kstatus/observe/common"
	"sigs.k8s.io/kustomize/kstatus/status"
	"sigs.k8s.io/kustomize/kstatus/wait"
)

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