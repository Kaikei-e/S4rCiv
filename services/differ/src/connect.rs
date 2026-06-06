//! Hand-rolled Connect-unary endpoint for `DiffService.ComputeChange`.
//!
//! Connect unary wire contract for a single unary RPC (no gRPC framing):
//! - request: `POST /s4rciv.diff.v1.DiffService/ComputeChange`,
//!   `Content-Type: application/proto`, body = prost-encoded `ComputeChangeRequest`
//!   (raw message, NO 5-byte gRPC length prefix).
//! - response: HTTP 200, `Content-Type: application/proto`, body = prost-encoded
//!   `ComputeChangeResponse`.
//! - error: non-200 with a JSON Connect error body `{"code":..,"message":..}`.
//!
//! A `GET /healthz` returns 200 for liveness checks.

use axum::Router;
use axum::body::Bytes;
use axum::extract::DefaultBodyLimit;
use axum::http::{HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use prost::Message;
use serde::Serialize;

use crate::diff::{self, ChangeOp, Classification, Confidence};
use crate::proto::diff::v1::{
    ChangeOp as PbChangeOp, ComputeChangeRequest, ComputeChangeResponse, NodeChange as PbNodeChange,
};
use crate::xmlmodel::{self, NodeType};

const CONTENT_TYPE_PROTO: &str = "application/proto";

/// Connect error envelope serialized as the JSON body on non-200 responses.
#[derive(Debug, Serialize)]
struct ConnectError {
    code: &'static str,
    message: String,
}

impl ConnectError {
    fn invalid_argument(message: impl Into<String>) -> (StatusCode, Self) {
        (
            StatusCode::BAD_REQUEST,
            Self {
                code: "invalid_argument",
                message: message.into(),
            },
        )
    }

    fn internal(message: impl Into<String>) -> (StatusCode, Self) {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Self {
                code: "internal",
                message: message.into(),
            },
        )
    }
}

/// Explicit request-body ceiling (CWE-400). The collector caps each fetched
/// snapshot at 64 MiB and a prev+curr 法令標準XML pair is far smaller in practice,
/// so this is a generous-but-bounded limit that makes the resource bound explicit
/// and tunable instead of relying on axum's implicit 2 MiB default.
const MAX_REQUEST_BYTES: usize = 64 * 1024 * 1024;

/// Build the router. No shared state: the service is fully stateless.
pub fn router() -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route(
            "/s4rciv.diff.v1.DiffService/ComputeChange",
            post(compute_change),
        )
        .layer(DefaultBodyLimit::max(MAX_REQUEST_BYTES))
}

async fn healthz() -> StatusCode {
    StatusCode::OK
}

/// Connect-unary handler: decode the proto request, run the diff, encode the proto
/// response. Errors become a Connect JSON error body with a matching HTTP status.
async fn compute_change(body: Bytes) -> Response {
    let request = match ComputeChangeRequest::decode(body) {
        Ok(req) => req,
        Err(e) => {
            return error_response(ConnectError::invalid_argument(format!(
                "failed to decode ComputeChangeRequest: {e}"
            )));
        }
    };

    match handle(&request) {
        Ok(resp) => proto_response(&resp),
        Err((status, err)) => error_response((status, err)),
    }
}

/// Pure request→response logic, separated from the HTTP layer for testability.
fn handle(
    request: &ComputeChangeRequest,
) -> Result<ComputeChangeResponse, (StatusCode, ConnectError)> {
    let prev = xmlmodel::parse(&request.prev_snapshot)
        .map_err(|e| ConnectError::internal(format!("prev_snapshot parse failed: {e}")))?;
    let curr = xmlmodel::parse(&request.curr_snapshot)
        .map_err(|e| ConnectError::internal(format!("curr_snapshot parse failed: {e}")))?;

    let result = diff::compute(&prev, &curr);

    let node_changes = result
        .changes
        .into_iter()
        .map(|c| PbNodeChange {
            eid: c.eid,
            op: map_op(c.op) as i32,
            node_type: map_node_type(c.node_type).to_string(),
            num: c.num,
            prev_text: c.prev_text,
            curr_text: c.curr_text,
        })
        .collect();

    Ok(ComputeChangeResponse {
        differ_version: diff::DIFFER_VERSION.to_string(),
        classification: map_classification(result.classification).to_string(),
        class_confidence: map_confidence(result.confidence).to_string(),
        node_changes,
    })
}

fn map_op(op: ChangeOp) -> PbChangeOp {
    match op {
        ChangeOp::Added => PbChangeOp::Added,
        ChangeOp::Deleted => PbChangeOp::Deleted,
        ChangeOp::Modified => PbChangeOp::Modified,
        ChangeOp::Moved => PbChangeOp::Moved,
    }
}

fn map_node_type(t: NodeType) -> &'static str {
    t.as_str()
}

fn map_classification(c: Classification) -> &'static str {
    c.as_str()
}

fn map_confidence(c: Confidence) -> &'static str {
    c.as_str()
}

fn proto_response(resp: &ComputeChangeResponse) -> Response {
    let body = resp.encode_to_vec();
    (
        StatusCode::OK,
        [(
            header::CONTENT_TYPE,
            HeaderValue::from_static(CONTENT_TYPE_PROTO),
        )],
        body,
    )
        .into_response()
}

fn error_response((status, err): (StatusCode, ConnectError)) -> Response {
    let body = serde_json::to_vec(&err).unwrap_or_else(|_| {
        br#"{"code":"internal","message":"error serialization failed"}"#.to_vec()
    });
    (
        status,
        [(
            header::CONTENT_TYPE,
            HeaderValue::from_static("application/json"),
        )],
        body,
    )
        .into_response()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn connect_error_serializes_to_spec_shape() {
        let (_, err) = ConnectError::internal("boom");
        let json = serde_json::to_string(&err).unwrap();
        assert_eq!(json, r#"{"code":"internal","message":"boom"}"#);
    }

    #[test]
    fn handle_round_trips_proto() {
        let req = ComputeChangeRequest {
            stream_id: "egov-law:test".to_string(),
            media_type: "application/xml".to_string(),
            prev_snapshot: Vec::new(),
            curr_snapshot: Vec::new(),
        };
        let resp = handle(&req).expect("handle should succeed on empty snapshots");
        assert_eq!(resp.differ_version, diff::DIFFER_VERSION);
        // Empty vs empty → no changes, administrative.
        assert!(resp.node_changes.is_empty());
        assert_eq!(resp.classification, "administrative");
    }
}
