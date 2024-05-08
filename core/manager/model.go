package manager

import (
	"gotcc/core/component"
	"time"
)

type ComponentEntity struct {
	Request   map[string]interface{}
	Component component.TCCComponent
}

type ComponentEntities []*ComponentEntity

func (ces ComponentEntities) ToComponents() []component.TCCComponent {
	components := make([]component.TCCComponent, 0, len(ces))
	for _, ce := range ces {
		components = append(components, ce.Component)
	}
	return components
}

type RequestEntity struct {
	ComponentID string
	Request     map[string]interface{}
}

type TxStatus string

const (
	TxHanging    TxStatus = "hanging"
	TxSuccessful TxStatus = "successful"
	TxFailure    TxStatus = "failure"
)

type ComponentTryStatus string

const (
	TryHanging    ComponentTryStatus = "hanging"
	TrySuccessful ComponentTryStatus = "successful"
	TryFailure    ComponentTryStatus = "failure"
)

type ComponentTryEntity struct {
	ComponentID string
	TryStatus   ComponentTryStatus
}

type Transaction struct {
	TxID       string
	Components []*ComponentTryEntity
	Status     TxStatus
	CreatedAt  time.Time
}

func (t *Transaction) getStatus(createdBefore time.Time) TxStatus {
	if t.CreatedAt.Before(createdBefore) {
		return TxFailure
	}
	var hangingExist bool
	for _, component := range t.Components {
		if component.TryStatus == TryFailure {
			return TxFailure
		}
		hangingExist = hangingExist || (component.TryStatus != TrySuccessful)
	}
	if hangingExist {
		return TxHanging
	}
	return TxSuccessful
}
