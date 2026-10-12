# NudeDetector

API HTTP e CLI em Go para estimar a probabilidade de pornografia em imagens.
O resultado principal é `pornography_score = P(porn)`, obtido do classificador
multiclasse (`drawings`, `hentai`, `neutral`, `porn`, `sexy`). `sexy` e `hentai`
não são convertidos automaticamente em pornografia; essa decisão pertence à
política configurável.

## Requisitos

- Go 1.23 ou superior, GNU Make, `curl` e `sha256sum`.
- ONNX Runtime CPU 1.22.0 (baixado por `make onnxruntime` ou pelo Docker).
- FFmpeg somente para exportar GIFs e vídeos.

## Compilar e executar

```bash
make model onnxruntime build
./bin/nude-detector serve
```

`make run` executa todo o fluxo acima e inicia a API em `:8080`. O runtime
também pode ser indicado explicitamente:

```bash
ONNXRUNTIME_SHARED_LIBRARY_PATH=.cache/onnxruntime/1.22.0/lib/libonnxruntime.so.1.22.0 \
  ./bin/nude-detector serve --addr :9090
```

O startup emite logs estruturados JSON contendo versão, commit, build, modelo,
labels, runtime ONNX, detector e porta efetivamente utilizada.

## CLI

Todos os comandos aceitam as flags globais `--addr` e `--runtime-library`.

```bash
./bin/nude-detector --help
./bin/nude-detector serve --addr :8080
./bin/nude-detector classify --input ./foto.jpg
./bin/nude-detector export --input ./video.mp4 --output ./moderated.jpg \
  --draw-boxes --blur --blur-radius 25 --frame-time-ms 1000
./bin/nude-detector scan --input "$HOME/.sx" --output-dir ./scan-output \
  --recursive --draw-boxes --blur
./bin/nude-detector version
```

`classify` lê uma imagem local e escreve JSON no stdout. `export` aceita imagem,
GIF ou vídeo, usa o frame inicial por padrão, permite selecionar outro frame
com `--frame-time-ms`, desenha boxes por padrão e grava sempre JPEG. Flags de
`export`: `--input/-i`, `--output/-o`, `--draw-boxes` (padrão `true`), `--blur`,
`--blur-radius` (1–100, padrão `25`) e `--frame-time-ms` (padrão `0`).

`scan` percorre uma pasta, classifica imagens e exporta GIFs/vídeos para JPEG.
Ele mantém uma única sessão de inferência e escreve um resultado JSON por linha
no stdout. Para somente JSON, omita `--output-dir`; para gerar imagens JPEG,
informe `--output-dir`. Use `--json-output resultados.json` para gravar JSONL
em arquivo ou `--json-output -` para stdout. Flags: `--input/-i`,
`--output-dir/-o`, `--json-output`, `--recursive` (padrão `true`),
`--draw-boxes`, `--blur`, `--blur-radius` e `--frame-time-ms`. Arquivos não
suportados são ignorados.
Ao final, o CLI sempre emite um registro `{"type":"summary",...}`, inclusive
quando a pasta está vazia.

## API HTTP

Base URL padrão: `http://localhost:8080`.

### Classificar upload

```bash
curl -sS -X POST http://localhost:8080/v1/moderation/pornography \
  -F 'image=@./foto.jpg'
```

Para incluir as classes semânticas e as detecções NudeNet, envie `details=true`
no corpo multipart:

```bash
curl -sS -X POST http://localhost:8080/v1/moderation/pornography \
  -F 'image=@./foto.jpg' -F 'details=true'
```

### Classificar URL pública

```bash
curl -sS -X POST http://localhost:8080/v1/moderation/pornography \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/foto.jpg"}'
```

O mesmo campo funciona quando a origem é uma URL:

```bash
curl -sS -X POST http://localhost:8080/v1/moderation/pornography \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/foto.jpg","details":true}'
```

Upload e URL são alternativas mutuamente exclusivas. Sem `details: true`, a
resposta contém `pornography_score`, percentual, classificação
(`not_pornographic`, `review` ou `pornographic`), thresholds, versão do modelo
e indicação de cache. Com `details: true`, ela também contém todas as classes,
seus pesos e as detecções NudeNet:

```json
{
  "pornography_score": 0.9271,
  "pornography_percent": 92.71,
  "classification": "pornographic",
  "scores": {"drawings": 0.0028, "hentai": 0.0041, "neutral": 0.0148, "porn": 0.9271, "sexy": 0.0512},
  "details": [{"label": "FEMALE_BREAST_EXPOSED", "score": 0.92, "box": {"x": 12, "y": 20, "width": 180, "height": 220}}],
  "thresholds": {"review": 0.4, "pornographic": 0.8},
  "model": {"name": "nsfw-classifier", "version": "1.0.0"},
  "processing": {"cached": false}
}
```

### Exportar boxes, categorias e blur

```bash
curl -sS -X POST http://localhost:8080/v1/moderation/export \
  -F 'image=@./video.mp4' \
  -F 'draw_boxes=true' -F 'blur=true' -F 'blur_radius=25' \
  -F 'frame_time_ms=1000' -o moderated.jpg
```

A resposta é `image/jpeg`, inclui `X-NudeDetector-Detections` e usa o primeiro
frame (`frame_time_ms=0`) por padrão. Suporta JPEG, PNG, WebP, GIF e formatos
de vídeo aceitos pelo FFmpeg.

### Saúde, prontidão e métricas

```bash
curl -sS http://localhost:8080/health   # processo vivo
curl -sS http://localhost:8080/ready    # modelo/runtime prontos
curl -sS http://localhost:8080/metrics  # Prometheus
```

`/ready` só retorna `ready: true` após carregar, validar e aquecer o modelo.

## Configuração

As variáveis principais são: `HTTP_ADDR`, `CLASSIFIER_MODEL_PATH`,
`CLASSIFIER_METADATA_PATH`, `CLASSIFIER_MODEL_URL`, `CLASSIFIER_MODEL_SHA256`,
`ONNXRUNTIME_SHARED_LIBRARY_PATH`, `PORN_REVIEW_THRESHOLD`, `PORN_THRESHOLD`,
`CLASSIFY_HENTAI_AS_PORNOGRAPHY`, `MAX_IMAGE_BYTES`, `MAX_IMAGE_PIXELS`,
`MAX_WIDTH`, `MAX_HEIGHT`, `DOWNLOAD_TIMEOUT`, `DIAL_TIMEOUT`,
`TLS_HANDSHAKE_TIMEOUT`, `RESPONSE_HEADER_TIMEOUT`, `MAX_REDIRECTS`,
`INFERENCE_CONCURRENCY`, `MAX_PENDING_INFERENCES`, `ONNX_INTRA_OP_THREADS`,
`ONNX_INTER_OP_THREADS`, `CACHE_ENABLED`, `CACHE_MAX_ENTRIES`, `CACHE_TTL`,
`DETECTOR_ENABLED`, `DETECTOR_MODEL_PATH`, `DETECTOR_MODEL_URL`,
`DETECTOR_MODEL_SHA256`, `DETECTOR_SCORE_MINIMUM`, `DETECTOR_IOU_THRESHOLD` e
`PPROF_ENABLED`. Valores de threshold são números entre `0` e `1`.

## Erros e segurança

Erros usam `{ "error": { "code": "...", "message": "..." } }`. Os principais
status são `400` (requisição), `413` (tamanho), `415` (tipo), `422` (imagem),
`502` (falha de download) e `503` (sobrecarga). URLs passam por proteção contra
SSRF, redirects são revalidados e downloads/imagens possuem limites e timeout.
Imagens não são persistidas por padrão; o cache usa SHA-256 e versão do modelo.

## Docker e desenvolvimento

```bash
make docker
docker run --rm -p 8080:8080 nude-detector:latest
make test
make test-race
make vet
make check
make clean
```

O Dockerfile é multi-stage, instala apenas o runtime ONNX necessário na imagem
final, executa como usuário não-root e inclui labels OCI de versão, commit,
data de build e origem. `make all` roda verificações e compila.
