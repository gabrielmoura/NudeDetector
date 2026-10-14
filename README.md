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
# Carrega somente o classificador NSFW (padrão), sem inicializar NudeNet.
./bin/nude-detector serve --models nsfw-classifier
# Habilita explicitamente os detalhes NudeNet para a mesma instância.
./bin/nude-detector serve --models nsfw-classifier,nudenet-detector
./bin/nude-detector classify --input ./foto.jpg
./bin/nude-detector export --input ./video.mp4 --output ./moderated.jpg \
  --draw-boxes --blur --blur-radius 25 --frame-time-ms 1000
./bin/nude-detector scan --input "$HOME/.sx" --output-dir ./scan-output \
  --recursive --draw-boxes --blur
./bin/nude-detector version
```

### ASR local (Whisper)

ASR usa Sherpa-ONNX em CPU e não baixa modelos ao atender uma requisição. A
instalação deve conter os artefatos INT8 de uma revisão conhecida sob
`ASR_MODELS_DIR` (padrão `./models/asr`), por exemplo para Whisper Tiny:

```text
models/asr/whisper-tiny/tiny-encoder.int8.onnx
models/asr/whisper-tiny/tiny-decoder.int8.onnx
models/asr/whisper-tiny/tiny-tokens.txt
```

Os artefatos e a revisão são publicados pelo
[repositório oficial Sherpa Whisper Tiny](https://huggingface.co/csukuangfj/sherpa-onnx-whisper-tiny).
O perfil Tiny pode ser instalado com verificação de integridade:

```bash
./bin/nude-detector models install whisper-tiny
./bin/nude-detector models verify whisper-tiny
# Os perfis Base e Small são maiores e também são instalados somente sob demanda.
./bin/nude-detector models install whisper-base
./bin/nude-detector models install whisper-small
```
Após a instalação, o primeiro caminho efetivo é WAV PCM ou float:

```bash
./bin/nude-detector asr transcribe audio.wav --model whisper-tiny --language pt
./bin/nude-detector asr transcribe audio.wav --model whisper-tiny --format json --timestamps
./bin/nude-detector asr transcribe audio.mp3 --model whisper-tiny --format text --output resultado.txt
curl -sS -X POST http://localhost:8080/v1/audio/transcriptions \
  -F file=@audio.wav -F language=pt -F timestamps=true
```

Para a API, inicie com `--models whisper-tiny` (ou defina
`ENABLED_MODELS=whisper-tiny`). A resposta mantém `segments` vazio até o
modelo expor segmentos confiáveis; timestamps de token retornados pelo Sherpa
ficam em `metadata.token_timestamps_seconds` quando solicitados.

WAV PCM/float usa o decodificador embutido. MP3, OGG/Opus, M4A/AAC e FLAC são
normalizados por FFmpeg, chamado sem shell, para PCM mono a 16 kHz. Defina
`ASR_MAX_DURATION_SECONDS` (padrão: 7200) para limitar a duração aceita.
`ASR_CHUNK_SECONDS` (padrão: 30) limita a duração de cada inferência Whisper;
os textos adjacentes são reconciliados para remover sobreposição literal nas
fronteiras dos blocos.

### OCR local (PP-OCRv6)

OCR usa as duas redes ONNX do perfil PP-OCRv6 e funciona com JPEG, PNG e WebP.
Instale os artefatos de uma mesma revisão sob `OCR_MODELS_DIR` (padrão
`./models/ocr`):

```text
models/ocr/pp-ocrv6-tiny/det.onnx
models/ocr/pp-ocrv6-tiny/rec.onnx
models/ocr/pp-ocrv6-tiny/rec.yml
```

```bash
./bin/nude-detector ocr recognize imagem.png --model pp-ocrv6-tiny --boxes
./bin/nude-detector ocr recognize imagem.png --format json --boxes --output resultado.json
./bin/nude-detector serve --models pp-ocrv6-tiny
curl -sS -X POST http://localhost:8080/v1/ocr -F image=@imagem.png -F return_boxes=true
```

As coordenadas retornadas são absolutas, em pixels da imagem de entrada.
O perfil Tiny também possui instalação explícita e verificável:

```bash
./bin/nude-detector models install pp-ocrv6-tiny
./bin/nude-detector models verify pp-ocrv6-tiny
# O perfil Small usa os mesmos contratos de OCR, com artefatos maiores.
./bin/nude-detector models install pp-ocrv6-small
```

### Tradução local (OPUS-MT)

OPUS-MT usa encoder e decoder ONNX em CPU, tokenizer Hugging Face e geração
autoregressiva local. Não há chamada a serviço externo durante a inferência.
Cada perfil instalado precisa destes arquivos, sob `TRANSLATION_MODELS_DIR`
(padrão `./models/translation`):

```text
models/translation/opus-mt-en-romance/encoder.onnx
models/translation/opus-mt-en-romance/decoder.onnx
models/translation/opus-mt-en-romance/tokenizer.json
models/translation/opus-mt-en-romance/vocab.json
```

O instalador baixa somente quando solicitado, com limites, URLs HTTPS, SHA-256
e renomeação atômica. Ele não é acionado durante uma requisição ou no startup:

```bash
./bin/nude-detector models install opus-mt-en-romance
./bin/nude-detector models verify opus-mt-en-romance
./bin/nude-detector models install opus-mt-romance-en
./bin/nude-detector models remove opus-mt-en-romance
```

`verify` recalcula o SHA-256 para modelos com manifesto instalado; `remove`
remove somente os caminhos de artefatos explicitamente listados no manifesto.

Use a revisão fixa `9d2ba69ac80c8e8453c3d9a1e2323a0e7b8ca3cd` do
[Xenova/opus-mt-en-ROMANCE](https://huggingface.co/Xenova/opus-mt-en-ROMANCE)
para inglês para português, espanhol, francês ou italiano. Para a direção
inversa, use a revisão `f60feaabe8bb435a5871e7702b25f53a01e770f8` de
[Xenova/opus-mt-ROMANCE-en](https://huggingface.co/Xenova/opus-mt-ROMANCE-en).

Textos grandes são segmentados nos limites de palavras antes da geração. URLs,
tags HTML e trechos entre crases são preservados literalmente pelo adaptador
compartilhado de tradução.

Para pares entre os cinco idiomas suportados (`en`, `pt`, `es`, `fr`, `it`), o
perfil multilíngue M2M100 pode ser selecionado explicitamente. Ele é maior e
tem um limite de artefato de 1 GiB, portanto continua desabilitado e
descarregado por padrão:

```bash
./bin/nude-detector models install m2m100-418m
./bin/nude-detector models verify m2m100-418m
./bin/nude-detector translate 'Hello world' --from en --to pt --model m2m100-418m
./bin/nude-detector serve --models m2m100-418m
```

```bash
./bin/nude-detector translate 'Hello world' --from en --to pt
./bin/nude-detector translate --file texto.txt --from pt --to en --model opus-mt-romance-en --format json
./bin/nude-detector translate 'Hello world' --from en --to pt --output resultado.txt
./bin/nude-detector serve --models opus-mt-en-romance
curl -sS -X POST http://localhost:8080/v1/translations \
  -H 'Content-Type: application/json' \
  -d '{"text":"Hello world","source_language":"en","target_language":"pt"}'

# Lote: a resposta contém {"translations":[...]}.
curl -sS -X POST http://localhost:8080/v1/translations \
  -H 'Content-Type: application/json' \
  -d '{"texts":["Hello","Good morning"],"source_language":"en","target_language":"pt"}'
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

A referência completa, incluindo os contratos ASR, OCR, tradução, catálogo de
modelos e códigos de erro, está em [docs/api.md](docs/api.md). As capacidades
novas só ficam disponíveis quando seu modelo for instalado e selecionado em
`serve --models`; a seleção não carrega os demais modelos.

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
Além das métricas de moderação existentes, as capacidades novas expõem
`ai_requests_total`, `ai_inference_duration_seconds`, `ai_errors_total` e
`ai_inference_in_flight`, rotuladas somente por capacidade e ID de modelo.

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
`DETECTOR_MODEL_SHA256`, `DETECTOR_SCORE_MINIMUM`, `DETECTOR_IOU_THRESHOLD`,
`ENABLED_MODELS`, `MAX_AUDIO_BYTES`, `ASR_MODELS_DIR`, `ASR_THREADS`,
`ASR_MAX_DURATION_SECONDS`,
`OCR_MODELS_DIR`, `OCR_MAX_SIDE`, `TRANSLATION_MODELS_DIR`,
`TRANSLATION_MAX_INPUT_TOKENS`, `TRANSLATION_MAX_OUTPUT_TOKENS`,
`TRANSLATION_MAX_CHARACTERS`, `TRANSLATION_MAX_BATCH` e `PPROF_ENABLED`. Valores de threshold são números entre
`0` e `1`.

`ENABLED_MODELS` aceita IDs separados por vírgula e controla exclusivamente o
que o modo `serve` pode inicializar. O padrão é `nsfw-classifier`; portanto o
processo não baixa nem mantém a sessão NudeNet até que
`nudenet-detector` seja selecionado. A flag `serve --models` substitui a
variável de ambiente. Consulte `GET /v1/models` e `GET /v1/models/{id}` para
o catálogo, estado de instalação e seleção corrente.

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
