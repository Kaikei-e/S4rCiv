// Package diffrpc is the Connect-RPC client to the Rust differ's DiffService. It
// implements port.DiffClient: Go sends two consecutive canonical snapshots and
// receives the structural change to persist (ADR-000005 — the diff is computed by
// the differ, persistence is owned by Go).
package diffrpc

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"

	diffv1 "s4rciv.org/api/gen/s4rciv/diff/v1"
	"s4rciv.org/api/gen/s4rciv/diff/v1/diffv1connect"
	"s4rciv.org/api/internal/port"
)

type Client struct {
	svc diffv1connect.DiffServiceClient
}

// New builds a DiffService client over the Connect protocol (default) with the
// proto codec, against baseURL (e.g. http://differ:9090).
func New(baseURL string) *Client {
	httpc := &http.Client{Timeout: 60 * time.Second}
	return &Client{svc: diffv1connect.NewDiffServiceClient(httpc, baseURL)}
}

func (c *Client) ComputeChange(ctx context.Context, prev, curr []byte, streamID, mediaType string) (port.DiffResult, error) {
	resp, err := c.svc.ComputeChange(ctx, connect.NewRequest(&diffv1.ComputeChangeRequest{
		StreamId:     streamID,
		MediaType:    mediaType,
		PrevSnapshot: prev,
		CurrSnapshot: curr,
	}))
	if err != nil {
		return port.DiffResult{}, err
	}
	msg := resp.Msg
	out := port.DiffResult{
		DifferVersion:   msg.GetDifferVersion(),
		Classification:  msg.GetClassification(),
		ClassConfidence: msg.GetClassConfidence(),
	}
	for _, nc := range msg.GetNodeChanges() {
		out.NodeChanges = append(out.NodeChanges, port.NodeChange{
			EID:      nc.GetEid(),
			Op:       opString(nc.GetOp()),
			NodeType: nc.GetNodeType(),
			Num:      nc.GetNum(),
			PrevText: nc.GetPrevText(),
			CurrText: nc.GetCurrText(),
		})
	}
	return out, nil
}

// opString maps the ChangeOp enum to the textual op stored in the diff JSON.
func opString(op diffv1.ChangeOp) string {
	switch op {
	case diffv1.ChangeOp_CHANGE_OP_ADDED:
		return "added"
	case diffv1.ChangeOp_CHANGE_OP_DELETED:
		return "deleted"
	case diffv1.ChangeOp_CHANGE_OP_MODIFIED:
		return "modified"
	case diffv1.ChangeOp_CHANGE_OP_MOVED:
		return "moved"
	default:
		return ""
	}
}
