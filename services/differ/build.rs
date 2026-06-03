//! Compile the vendored diff proto into prost message types.
//!
//! This invokes `protoc` (assumed on PATH) via `prost-build` and writes the
//! generated Rust into `OUT_DIR`. We only need message + enum types here — there
//! is no gRPC service stub, because the Connect-unary endpoint is hand-rolled in
//! `src/connect.rs`.

use std::io::Result;

fn main() -> Result<()> {
    let proto = "proto/s4rciv/diff/v1/diff.proto";
    println!("cargo:rerun-if-changed={proto}");
    prost_build::compile_protos(&[proto], &["proto"])?;
    Ok(())
}
