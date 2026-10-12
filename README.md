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
- `POST /v1/moderation/export` devolve JPEG com boxes/categorias e blur opcional
- GIFs e vídeos usam FFmpeg para extrair o primeiro frame (ou `frame_time_ms`)

## Execução
```bash
ONNXRUNTIME_SHARED_LIBRARY_PATH=/path/to/libonnxruntime.so.1.22.0 \
CLASSIFIER_MODEL_PATH=./models/model.onnx CLASSIFIER_METADATA_PATH=./models/model.json \
INFERENCE_CONCURRENCY=4 MAX_PENDING_INFERENCES=100 ./nude-detector
```

Use ONNX Runtime 1.22.0 or a compatible newer release: the bundled model uses
ONNX opset 22. The Docker image downloads the pinned CPU runtime automatically.

## Exportação anotada

Envie `image` via multipart para `POST /v1/moderation/export`. `draw_boxes=true`
é o padrão; use `blur=true`, `blur_radius=25` e, para GIF/vídeo,
`frame_time_ms=0` (primeiro frame). A resposta é `image/jpeg`.

## CLI

O mesmo binário oferece API e CLI local via Cobra:

```bash
# inicia a API
./nude-detector serve

# classifica uma imagem e escreve JSON no stdout
./nude-detector classify --input image.jpg

# gera JPEG com boxes, categorias e blur opcional
./nude-detector export --input video.mp4 --output moderated.jpg --blur --frame-time-ms 1000
```

Na ausência de `CLASSIFIER_MODEL_PATH` ou `DETECTOR_MODEL_PATH`, os modelos
fixados são baixados e validados por SHA-256 no startup. Um caminho existente,
inclusive montado no container, é sempre utilizado sem novo download.

## Segurança
- Bloqueia localhost/loopback, ranges privados, CGNAT, IPv6 link-local
- Limites: MAX_IMAGE_BYTES, MAX_IMAGE_PIXELS, MAX_WIDTH, MAX_HEIGHT
- Timeouts de download configuráveis
