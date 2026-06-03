//! End-to-end wire test for the Connect-unary endpoint: spin up the real router on
//! an ephemeral port and exercise it with a raw HTTP client (hand-written request
//! over a TCP socket, so the test depends on no extra HTTP-client crate).

use std::io::{Read, Write};
use std::net::{Ipv4Addr, SocketAddr, TcpStream};
use std::time::Duration;

use differ::connect;
use differ::proto::diff::v1::{ComputeChangeRequest, ComputeChangeResponse};
use prost::Message;

async fn spawn_server() -> SocketAddr {
    let listener = tokio::net::TcpListener::bind(SocketAddr::from((Ipv4Addr::LOCALHOST, 0)))
        .await
        .expect("bind ephemeral port");
    let addr = listener.local_addr().unwrap();
    tokio::spawn(async move {
        axum::serve(listener, connect::router()).await.unwrap();
    });
    addr
}

/// Issue a raw HTTP request and return (status_line, body_bytes).
fn raw_request(addr: SocketAddr, request: &[u8]) -> (String, Vec<u8>) {
    let mut stream = TcpStream::connect(addr).expect("connect");
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("set read timeout");
    stream.write_all(request).expect("write request");
    // With `Connection: close` the server closes the socket after responding, which
    // gives us a clean EOF; the read timeout guards against a hang.
    let mut raw = Vec::new();
    stream.read_to_end(&mut raw).expect("read response");

    let split = raw
        .windows(4)
        .position(|w| w == b"\r\n\r\n")
        .expect("response has header/body separator");
    let head = String::from_utf8_lossy(&raw[..split]).to_string();
    let body = raw[split + 4..].to_vec();
    let status_line = head.lines().next().unwrap_or_default().to_string();
    (status_line, body)
}

/// Run the blocking raw-socket client off the async runtime so it cannot starve
/// the in-process server task.
async fn request(addr: SocketAddr, http: Vec<u8>) -> (String, Vec<u8>) {
    tokio::task::spawn_blocking(move || raw_request(addr, &http))
        .await
        .expect("client task")
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn healthz_returns_200() {
    let addr = spawn_server().await;
    let req = format!("GET /healthz HTTP/1.1\r\nHost: {addr}\r\nConnection: close\r\n\r\n");
    let (status, _) = request(addr, req.into_bytes()).await;
    assert!(status.contains("200"), "healthz status was {status}");
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn compute_change_round_trips_proto_over_the_wire() {
    let addr = spawn_server().await;

    let curr = r#"<Law><LawBody><MainProvision>
      <Article Num="1">
        <ArticleTitle>第一条</ArticleTitle>
        <Paragraph Num="1">
          <ParagraphSentence><Sentence>本則。</Sentence></ParagraphSentence>
        </Paragraph>
      </Article>
    </MainProvision></LawBody></Law>"#;

    let proto_req = ComputeChangeRequest {
        stream_id: "egov-law:wire".to_string(),
        media_type: "application/xml".to_string(),
        prev_snapshot: Vec::new(),
        curr_snapshot: curr.as_bytes().to_vec(),
    }
    .encode_to_vec();

    let mut http = Vec::new();
    http.extend_from_slice(b"POST /s4rciv.diff.v1.DiffService/ComputeChange HTTP/1.1\r\n");
    http.extend_from_slice(format!("Host: {addr}\r\n").as_bytes());
    http.extend_from_slice(b"Content-Type: application/proto\r\n");
    http.extend_from_slice(format!("Content-Length: {}\r\n", proto_req.len()).as_bytes());
    http.extend_from_slice(b"Connection: close\r\n\r\n");
    http.extend_from_slice(&proto_req);

    let (status, body) = request(addr, http).await;
    assert!(status.contains("200"), "status was {status}");

    let resp = ComputeChangeResponse::decode(body.as_slice()).expect("decode proto response");
    assert_eq!(resp.differ_version, "egov-akn-diff/0.1.0");
    // Empty prev → everything added; the paragraph carries text → substantive.
    assert_eq!(resp.classification, "substantive");
    assert!(resp.node_changes.iter().any(|c| c.eid == "art_1__para_1"));
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn malformed_proto_body_returns_connect_error() {
    let addr = spawn_server().await;
    let bad = b"\xff\xff\xff\xff not a proto";

    let mut http = Vec::new();
    http.extend_from_slice(b"POST /s4rciv.diff.v1.DiffService/ComputeChange HTTP/1.1\r\n");
    http.extend_from_slice(format!("Host: {addr}\r\n").as_bytes());
    http.extend_from_slice(b"Content-Type: application/proto\r\n");
    http.extend_from_slice(format!("Content-Length: {}\r\n", bad.len()).as_bytes());
    http.extend_from_slice(b"Connection: close\r\n\r\n");
    http.extend_from_slice(bad);

    let (status, body) = request(addr, http).await;
    assert!(
        status.contains("400"),
        "malformed proto should be 400, was {status}"
    );
    let text = String::from_utf8_lossy(&body);
    assert!(
        text.contains("\"code\":\"invalid_argument\""),
        "expected connect error body, got {text}"
    );
}
