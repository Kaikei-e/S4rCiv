package queryrpc

import (
	"context"
	"errors"
	"log"

	"connectrpc.com/connect"
)

// SanitizeErrors is a unary interceptor that prevents internal error detail from
// crossing the RPC boundary to the BFF/browser (CWE-209). For Internal/Unknown
// codes it logs the full error server-side and returns a generic, fixed message;
// the raw pgx/driver text (SQLSTATE, query structure, internal hostnames) never
// reaches the client. InvalidArgument / NotFound carry only caller-supplied
// identifiers, so they pass through unchanged.
func SanitizeErrors() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}
			if code := connect.CodeOf(err); code == connect.CodeInternal || code == connect.CodeUnknown {
				log.Printf("rpc %s failed: %v", req.Spec().Procedure, err)
				return resp, connect.NewError(code, errors.New("internal error"))
			}
			return resp, err
		}
	}
}
