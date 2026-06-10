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

use std::time::Duration;

use axum::Router;
use axum::body::Bytes;
use axum::extract::DefaultBodyLimit;
use axum::http::{HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use prost::Message;
use serde::Serialize;
use tower::limit::GlobalConcurrencyLimitLayer;

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

    fn deadline_exceeded(message: impl Into<String>) -> (StatusCode, Self) {
        (
            // Connect protocol maps deadline_exceeded to HTTP 504.
            StatusCode::GATEWAY_TIMEOUT,
            Self {
                code: "deadline_exceeded",
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

/// Per-request wall-clock budget for decode + parse + diff (CWE-400). Real
/// snapshot pairs finish in well under a second; a pathological-but-within-limit
/// body must not pin a worker indefinitely.
const REQUEST_TIMEOUT: Duration = Duration::from_secs(30);

/// Maximum `ComputeChange` requests processed at once. The work is pure CPU on
/// up to 64 MiB bodies, so this bounds peak CPU and memory; excess requests
/// queue on the shared semaphore instead of failing.
const MAX_CONCURRENT_REQUESTS: usize = 4;

/// Build the router. No shared state: the service is fully stateless.
pub fn router() -> Router {
    Router::new()
        .route("/healthz", get(healthz))
        .route(
            "/s4rciv.diff.v1.DiffService/ComputeChange",
            // The concurrency limit sits on the RPC route only, so liveness
            // checks stay responsive while diff workers are saturated.
            post(compute_change).layer(GlobalConcurrencyLimitLayer::new(MAX_CONCURRENT_REQUESTS)),
        )
        .layer(DefaultBodyLimit::max(MAX_REQUEST_BYTES))
}

async fn healthz() -> StatusCode {
    StatusCode::OK
}

/// Connect-unary handler: decode the proto request, run the diff, encode the proto
/// response. Errors become a Connect JSON error body with a matching HTTP status.
async fn compute_change(body: Bytes) -> Response {
    compute_change_with_budget(body, REQUEST_TIMEOUT).await
}

/// Budget-parameterized core, separated so tests can exercise the deadline path
/// without waiting out the real `REQUEST_TIMEOUT`.
///
/// Decode + parse + diff is pure CPU on up to 64 MiB, so it runs on the blocking
/// pool (keeping the async runtime responsive) under a wall-clock budget.
async fn compute_change_with_budget(body: Bytes, budget: Duration) -> Response {
    let work = tokio::task::spawn_blocking(move || {
        let request = ComputeChangeRequest::decode(body).map_err(|e| {
            ConnectError::invalid_argument(format!("failed to decode ComputeChangeRequest: {e}"))
        })?;
        handle(&request)
    });

    match tokio::time::timeout(budget, work).await {
        Ok(Ok(Ok(resp))) => proto_response(&resp),
        Ok(Ok(Err((status, err)))) => error_response((status, err)),
        Ok(Err(join_err)) => error_response(ConnectError::internal(format!(
            "diff worker failed: {join_err}"
        ))),
        Err(_elapsed) => error_response(ConnectError::deadline_exceeded(format!(
            "ComputeChange exceeded the {}s request budget",
            budget.as_secs()
        ))),
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
    use std::fmt::Write as _;

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

    #[test]
    fn deadline_exceeded_maps_to_gateway_timeout() {
        let (status, err) = ConnectError::deadline_exceeded("too slow");
        assert_eq!(status, StatusCode::GATEWAY_TIMEOUT);
        assert_eq!(err.code, "deadline_exceeded");
    }

    /// A valid request whose snapshot is big enough that the blocking worker
    /// cannot finish before an already-expired deadline is observed.
    fn bulky_request_body() -> Bytes {
        let mut xml = String::from(r#"<Law><LawBody><MainProvision><Article Num="1">"#);
        for i in 1..=5_000 {
            write!(
                xml,
                "<Paragraph Num=\"{i}\"><ParagraphSentence><Sentence>第{i}項。</Sentence></ParagraphSentence></Paragraph>"
            )
            .expect("write to String");
        }
        xml.push_str("</Article></MainProvision></LawBody></Law>");
        let req = ComputeChangeRequest {
            stream_id: "egov-law:budget".to_string(),
            media_type: "application/xml".to_string(),
            prev_snapshot: Vec::new(),
            curr_snapshot: xml.into_bytes(),
        };
        Bytes::from(req.encode_to_vec())
    }

    #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
    async fn expired_budget_returns_deadline_exceeded() {
        let resp = compute_change_with_budget(bulky_request_body(), Duration::ZERO).await;
        assert_eq!(resp.status(), StatusCode::GATEWAY_TIMEOUT);
    }

    #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
    async fn generous_budget_completes_off_the_runtime() {
        let resp = compute_change_with_budget(bulky_request_body(), REQUEST_TIMEOUT).await;
        assert_eq!(resp.status(), StatusCode::OK);
    }
}
