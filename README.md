# NudeDetector API

API de detecção de pornografia em imagens usando Go + ONNX Runtime.

## Objetivo
Estimar a probabilidade de pornografia (`pornography_score = P(porn)`) com base em classificador multiclasses (drawings, hentai, neutral, porn, sexy).

## Características
- POST /v1/moderation/pornography (multipart `image` ou JSON `{"url": ...}`)
- SSRF protegido (validação de IPs privados, redirects limitados)
- Concorrência limitada + backpressure
- Cache em memória por hash+versão modelo
- Métricas Prometheus (/metrics), health/ready
- Política separada do modelo (pornography_score usa apenas classe `porn`)

## Execução
```bash
ONNXRUNTIME_SHARED_LIBRARY_PATH=/path/to/libonnxruntime.so.1.22.0 \
CLASSIFIER_MODEL_PATH=./models/model.onnx CLASSIFIER_METADATA_PATH=./models/model.json \
INFERENCE_CONCURRENCY=4 MAX_PENDING_INFERENCES=100 ./nude-detector
```

Use ONNX Runtime 1.22.0 or a compatible newer release: the bundled model uses
ONNX opset 22. The Docker image downloads the pinned CPU runtime automatically.

## Segurança
- Bloqueia localhost/loopback, ranges privados, CGNAT, IPv6 link-local
- Limites: MAX_IMAGE_BYTES, MAX_IMAGE_PIXELS, MAX_WIDTH, MAX_HEIGHT
- Timeouts de download configuráveis
