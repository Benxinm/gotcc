package manager

import (
	"context"
	"errors"
	"fmt"
	"gotcc/core/component"
	"sync"
	"time"
)

type TxManager struct {
	ctx            context.Context
	stop           context.CancelFunc
	opts           *Options
	registerCenter *registerCenter
	txStore        TXStore
}

func NewTxManager(txStore TXStore, opts ...Option) *TxManager {
	ctx, cancel := context.WithCancel(context.Background())
	txManager := TxManager{
		opts:           &Options{},
		txStore:        txStore,
		registerCenter: newRegisterCenter(),
		ctx:            ctx,
		stop:           cancel,
	}

	for _, opt := range opts {
		//Inject parameter
		opt(txManager.opts)
	}

	repair(txManager.opts)
	go txManager.run()
	return &txManager
}

func (t *TxManager) Register(component component.TCCComponent) error {
	return t.registerCenter.register(component)
}

func (t *TxManager) Transaction(ctx context.Context, reqs ...*RequestEntity) (bool, error) {
	//limit running time
	tctx, cancel := context.WithTimeout(ctx, t.opts.Timeout)
	defer cancel()

	//get all related tcc components
	componentEntities, err := t.getComponents(ctx, reqs...)
	if err != nil {
		return false, err
	}
	//create tx record and get Guid
	txID, err := t.txStore.CreateTX(tctx, componentEntities.ToComponents()...)
	if err != nil {
		return false, err
	}
	return t.twoPhaseCommit(ctx, txID, componentEntities)
}

func (t *TxManager) twoPhaseCommit(ctx context.Context, txID string, entities ComponentEntities) (bool, error) {
	//create sub context to manage sub goroutine lifecycle
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	//create a chan to receive sub goroutines' error
	errCh := make(chan error)

	go func() {
		var wg sync.WaitGroup
		for _, e := range entities {
			e := e
			wg.Add(1)
			go func() {
				defer wg.Done()
				resp, err := e.Component.Try(cctx, &component.TCCReq{
					ComponentID: e.Component.ID(),
					TxID:        txID,
					Data:        e.Request,
				})
				//when tcc component try phase fail, we need to update tx
				if err != nil || !resp.Ack {
					_ = t.txStore.TxUpdate(cctx, txID, e.Component.ID(), true)
					errCh <- fmt.Errorf("component: %s try failed", e.Component.ID())
					return
				}
				//try phase success
				if err = t.txStore.TxUpdate(cctx, txID, e.Component.ID(), true); err != nil {
					errCh <- err
				}
			}()
		}
		wg.Wait()
		close(errCh)
	}()

	successful := true
	//if there is any one sub goroutine meet error, then it will receive error in advance
	if err := <-errCh; err != nil {
		cancel()
		successful = false
	}

	return successful, nil
}

func (t *TxManager) advanceProgressByTxID(txID string) error {
	tx, err := t.txStore.GetTx(t.ctx, txID)
	if err != nil {
		return err
	}
	return t.advanceProgress(tx)
}

func (t *TxManager) advanceProgress(tx *Transaction) error {
	txStatus := tx.getStatus(time.Now().Add(-t.opts.Timeout))
	//hanging status
	if txStatus == TxHanging {
		return nil
	}

	success := txStatus == TxSuccessful
	var confirmOrCancel func(ctx context.Context, component component.TCCComponent) (*component.TCCResp, error)
	var txAdvanceProgress func(ctx context.Context) error
	if success {
		confirmOrCancel = func(ctx context.Context, component component.TCCComponent) (*component.TCCResp, error) {
			return component.Confirm(ctx, tx.TxID)
		}
		txAdvanceProgress = func(ctx context.Context) error {
			return t.txStore.TxSubmit(ctx, tx.TxID, true)
		}
	} else {
		confirmOrCancel = func(ctx context.Context, component component.TCCComponent) (*component.TCCResp, error) {
			return component.Cancel(ctx, tx.TxID)
		}
		txAdvanceProgress = func(ctx context.Context) error {
			return t.txStore.TxSubmit(ctx, tx.TxID, false)
		}
	}

	for _, component := range tx.Components {
		components, err := t.registerCenter.getComponents(component.ComponentID)
		if err != nil || len(components) == 0 {
			return errors.New("get tcc component failed")
		}
		resp, err := confirmOrCancel(t.ctx, components[0])
		if err != nil {
			return err
		}
		if !resp.Ack {
			return fmt.Errorf("component: %s ack failed", component.ComponentID)
		}
	}

	return txAdvanceProgress(t.ctx)
}

func (t *TxManager) getComponents(ctx context.Context, reqs ...*RequestEntity) (ComponentEntities, error) {
	if len(reqs) == 0 {
		return nil, errors.New("empty task")
	}
	//record the map from id to request
	idToReq := make(map[string]*RequestEntity, len(reqs))
	componentIDs := make([]string, 0, len(reqs))
	for _, req := range reqs {
		if _, ok := idToReq[req.ComponentID]; ok {
			return nil, fmt.Errorf("repeat component: %s", req.ComponentID)
		}
		idToReq[req.ComponentID] = req
		componentIDs = append(componentIDs, req.ComponentID)
	}

	//validation
	components, err := t.registerCenter.getComponents(componentIDs...)
	if err != nil {
		return nil, err
	}
	if len(componentIDs) != len(components) {
		return nil, errors.New("invalid componentIDs")
	}

	entities := make(ComponentEntities, 0, len(components))
	for _, component := range components {
		entities = append(entities, &ComponentEntity{
			Request:   idToReq[component.ID()].Request,
			Component: component,
		})
	}
	return entities, nil
}

func (t *TxManager) batchAdvanceProgress(txs []*Transaction) error {
	errCh := make(chan error)
	go func() {
		var wg sync.WaitGroup
		for _, tx := range txs {
			tx := tx
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := t.advanceProgress(tx); err != nil {
					errCh <- err
				}
			}()
		}
		wg.Wait()
		close(errCh)
	}()

	var firstErr error
	for err := range errCh {
		if firstErr != nil {
			continue
		}
		firstErr = err
	}
	return firstErr
}

func (t *TxManager) backOffTick(tick time.Duration) time.Duration {
	tick <<= 1
	if threshold := t.opts.MonitorTick << 3; tick > threshold {
		return threshold
	}
	return tick
}

func (t *TxManager) run() {
	var tick time.Duration
	var err error
	for {
		if err == nil {
			tick = t.opts.MonitorTick
		} else {
			tick = t.backOffTick(tick)
		}
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(tick):
			if err = t.txStore.Lock(t.ctx, t.opts.MonitorTick); err != nil {
				err = nil
				continue
			}

			var txs []*Transaction
			if txs, err = t.txStore.GetHangingTxs(t.ctx); err != nil {
				_ = t.txStore.Unlock(t.ctx)
				continue
			}
			err = t.batchAdvanceProgress(txs)
			_ = t.txStore.Unlock(t.ctx)
		}
	}
}
