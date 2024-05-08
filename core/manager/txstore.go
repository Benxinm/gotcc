package manager

import (
	"context"
	"gotcc/core/component"
	"time"
)

type TXStore interface {
	// CreateTX create a transaction record
	CreateTX(ctx context.Context, components ...component.TCCComponent) (txID string, err error)
	// Lock log table
	Lock(ctx context.Context, expireDuration time.Duration) error
	// Unlock log table
	Unlock(ctx context.Context) error
	// TxUpdate update tx
	TxUpdate(ctx context.Context, txID string, componentID string, accept bool) error
	// GetTx get tx by id
	GetTx(ctx context.Context, txID string) (*Transaction, error)
	//TxSubmit submit tx
	TxSubmit(ctx context.Context, txID string, success bool) error
	//GetHangingTxs get hanging tx
	GetHangingTxs(ctx context.Context) ([]*Transaction, error)
}
