//! Generated prost types for the `s4rciv.diff.v1` package.
//!
//! `build.rs` writes the codegen into `OUT_DIR`; we include it here under the
//! package path so the rest of the crate refers to `proto::diff::v1::*`.

pub mod diff {
    pub mod v1 {
        include!(concat!(env!("OUT_DIR"), "/s4rciv.diff.v1.rs"));
    }
}
