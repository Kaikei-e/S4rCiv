//! Differ service entry point: wire config + router, bind, serve with graceful
//! shutdown. All logic lives in the library crate.

use std::net::{Ipv4Addr, SocketAddr};

use differ::config::Config;
use differ::connect;
use tracing::info;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    let config = Config::from_env();
    let addr = SocketAddr::from((Ipv4Addr::UNSPECIFIED, config.port));

    let app = connect::router();
    let listener = tokio::net::TcpListener::bind(addr).await?;
    info!(%addr, version = differ::diff::DIFFER_VERSION, "differ listening");

    axum::serve(listener, app)
        .with_graceful_shutdown(async {
            let _ = tokio::signal::ctrl_c().await;
            info!("received SIGINT, shutting down");
        })
        .await?;

    Ok(())
}
