# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS builder

WORKDIR /app

ARG ONNXRUNTIME_VERSION=1.22.0
ARG MODEL_URL=https://huggingface.co/onnx-community/nsfw-classifier-ONNX/resolve/main/onnx/model.onnx
ARG MODEL_SHA256=d6e841f61f193d9f435b464748c772b4096edd941c9f12b3a05bcb655fb267eb

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl \
	&& rm -rf /var/lib/apt/lists/*

RUN curl --fail --location --silent --show-error \
		"https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}.tgz" \
		--output /tmp/onnxruntime.tgz \
	&& tar -xzf /tmp/onnxruntime.tgz -C /tmp \
	&& install -Dm755 \
		"/tmp/onnxruntime-linux-x64-${ONNXRUNTIME_VERSION}/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION}" \
		"/out/lib/libonnxruntime.so.${ONNXRUNTIME_VERSION}"

RUN curl --fail --location --silent --show-error "$MODEL_URL" --output /out/model.onnx \
	&& echo "$MODEL_SHA256  /out/model.onnx" | sha256sum --check --status

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 go build -trimpath -buildvcs=false -o /out/nude-detector ./cmd/api

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/nude-detector /app/nude-detector
COPY --from=builder /out/lib /app/lib
COPY --from=builder /out/model.onnx /app/models/model.onnx
COPY models/model.json /app/models/model.json

ENV ONNXRUNTIME_SHARED_LIBRARY_PATH=/app/lib/libonnxruntime.so.1.22.0

USER 1000:1000
EXPOSE 8080
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
	CMD curl --fail --silent http://127.0.0.1:8080/ready || exit 1
ENTRYPOINT ["/app/nude-detector"]
