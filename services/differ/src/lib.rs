//! Stateless structural differ for Japanese law standard XML (法令標準XML).
//!
//! Exposes a single Connect-unary RPC, `s4rciv.diff.v1.DiffService.ComputeChange`,
//! which parses two canonical-XML snapshots into AKN-eId-keyed node maps, diffs
//! them at node granularity, and classifies the change as administrative or
//! substantive. The service holds NO state and never touches a database
//! (ADR-000005); the Go caller owns all persistence.

pub mod config;
pub mod connect;
pub mod diff;
pub mod proto;
pub mod xmlmodel;
