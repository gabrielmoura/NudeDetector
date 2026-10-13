# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS builder

WORKDIR /app

ARG ONNXRUNTIME_VERSION=1.22.0
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG MODEL_URL=https://huggingface.co/onnx-community/nsfw-classifier-ONNX/resolve/main/onnx/model.onnx
ARG MODEL_SHA256=d6e841f61f193d9f435b464748c772b4096edd941c9f12b3a05bcb655fb267eb
ARG DETECTOR_MODEL_URL=https://raw.githubusercontent.com/notAI-tech/NudeNet/v3/nudenet/320n.onnx
ARG DETECTOR_MODEL_SHA256=c15d8273adad2d0a92f014cc69ab2d6c311a06777a55545f2c4eb46f51911f0f

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
RUN curl --fail --location --silent --show-error "$DETECTOR_MODEL_URL" --output /out/320n.onnx \
	&& echo "$DETECTOR_MODEL_SHA256  /out/320n.onnx" | sha256sum --check --status

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 go build -trimpath -buildvcs=false \
	-ldflags "-X github.com/gabrielmoura/NudeDetector/internal/buildinfo.Version=${VERSION} -X github.com/gabrielmoura/NudeDetector/internal/buildinfo.Commit=${COMMIT} -X github.com/gabrielmoura/NudeDetector/internal/buildinfo.BuildTime=${BUILD_TIME}" \
	-o /out/nude-detector ./cmd/api

FROM debian:bookworm-slim

WORKDIR /app

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

LABEL org.opencontainers.image.title="NudeDetector" \
	org.opencontainers.image.description="Go and ONNX Runtime pornography moderation API and CLI" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.revision="${COMMIT}" \
	org.opencontainers.image.created="${BUILD_TIME}" \
	org.opencontainers.image.source="https://github.com/gabrielmoura/NudeDetector" \
	org.opencontainers.image.licenses="MIT"

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates curl ffmpeg \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/nude-detector /app/nude-detector
COPY --from=builder /out/lib /app/lib
COPY --from=builder /go/pkg/mod/github.com/k2-fsa/sherpa-onnx-go-linux@v1.13.8/lib/x86_64-unknown-linux-gnu /app/lib/sherpa
COPY --from=builder /out/model.onnx /app/models/model.onnx
COPY --from=builder /out/320n.onnx /app/models/320n.onnx
COPY models/model.json /app/models/model.json

ENV ONNXRUNTIME_SHARED_LIBRARY_PATH=/app/lib/libonnxruntime.so.1.22.0 \
    LD_LIBRARY_PATH=/app/lib/sherpa

USER 1000:1000
EXPOSE 8080
STOPSIGNAL SIGTERM
HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
	CMD curl --fail --silent http://127.0.0.1:8080/ready || exit 1
ENTRYPOINT ["/app/nude-detector"]
