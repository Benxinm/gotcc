package component

import "context"

type TCCReq struct {
	//Guid
	ComponentID string
	TxID        string
	Data        map[string]interface{}
}

type TCCResp struct {
	ComponentID string
	Ack         bool
	TxID        string
}

type TCCComponent interface {
	// ID return unique id
	ID() string
	Try(ctx context.Context, req *TCCReq) (*TCCResp, error)
	Confirm(ctx context.Context, txID string) (*TCCResp, error)
	Cancel(ctx context.Context, txId string) (*TCCResp, error)
}
