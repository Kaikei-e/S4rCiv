# syntax=docker/dockerfile:1
# Builder is Debian 12 (bookworm) to match the glibc of the distroless runtime.
FROM rust:1.95-bookworm AS build
WORKDIR /src
COPY Cargo.toml ./
COPY src ./src
RUN cargo build --release && cp target/release/differ /differ

FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /differ /app/differ
USER nonroot:nonroot
ENTRYPOINT ["/app/differ"]
