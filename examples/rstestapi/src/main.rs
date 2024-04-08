mod rstestapiv1 {
    tonic::include_proto!("rstestapi.v1");

    pub const FILE_DESCRIPTOR_SET: &[u8] =
        tonic::include_file_descriptor_set!("rstestapi.v1_descriptor");
}

use anyhow::{Context, Result};
use rstestapiv1::clock_service_server::{ClockService, ClockServiceServer};
use std::env;
use std::net::SocketAddr;
use std::time::SystemTime;
use tonic::{transport::Server, Request, Response, Status};
use tower::ServiceBuilder;
use tower_http::trace::{DefaultMakeSpan, DefaultOnRequest, DefaultOnResponse, TraceLayer};
use tracing::{info, Level};

#[derive(Debug, Default)]
pub struct ClockServiceImpl {}

#[tonic::async_trait]
impl ClockService for ClockServiceImpl {
    async fn get_time(&self, _: Request<()>) -> Result<Response<prost_types::Timestamp>, Status> {
        Ok(Response::new(SystemTime::now().into()))
    }
}

#[tokio::main]
async fn main() -> Result<()> {
    let json_subscriber = tracing_subscriber::fmt()
        .json()
        .with_max_level(Level::INFO)
        .flatten_event(true)
        .finish();
    tracing::subscriber::set_global_default(json_subscriber)
        .context("failed to set default tracing subscriber")?;

    let listen_addr: SocketAddr = env::var("LISTEN_ADDR")
        .unwrap_or("0.0.0.0:50053".to_owned())
        .parse()
        .map_err(anyhow::Error::new)
        .context("failed to read LISTEN_ADDR from env")?;

    let clock_service = ClockServiceImpl::default();

    // Currently tonic-reflection supports only v1alpha, which is great for testing grpcbridge!
    // In the future, if v1 gets added, make sure to manually disable it to avoid losing test coverage.
    let reflection_server = tonic_reflection::server::Builder::configure()
        .register_encoded_file_descriptor_set(rstestapiv1::FILE_DESCRIPTOR_SET)
        .build()?;

    info!(
        app = "rstestapi",
        listen_addr = listen_addr.to_string(),
        "serving test gRPC"
    );

    Server::builder()
        .layer(
            ServiceBuilder::new().layer(
                TraceLayer::new_for_grpc()
                    .make_span_with(DefaultMakeSpan::new().level(Level::INFO))
                    .on_request(DefaultOnRequest::new().level(Level::INFO))
                    .on_response(DefaultOnResponse::new().level(Level::INFO)),
            ),
        )
        .add_service(ClockServiceServer::new(clock_service))
        .add_service(reflection_server)
        .serve_with_shutdown(listen_addr, shutdown_signal())
        .await?;

    Ok(())
}

#[cfg(target_family = "unix")]
async fn shutdown_signal() {
    let mut sigterm =
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()).unwrap();
    let sigint = tokio::signal::ctrl_c();

    tokio::select! {
        _ = sigterm.recv() => {}
        _ = sigint => {}
    }
}

#[cfg(target_family = "windows")]
async fn shutdown_signal() {
    let _ = tokio::signal::ctrl_c().await;
}
