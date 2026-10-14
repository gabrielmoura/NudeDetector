# API HTTP

A API HTTP escuta em `HTTP_ADDR` (padrão `:8080`). Todas as capacidades de IA
são locais: um modelo deve ser instalado antecipadamente e selecionado quando
o processo é iniciado. Uma requisição nunca baixa um artefato de modelo.

## Seleção de modelos

`serve --models` (ou `ENABLED_MODELS`) recebe IDs separados por vírgula. Sem
essa opção, somente `nsfw-classifier` é carregado. O processo aceita no máximo
um modelo para cada capacidade ASR, OCR e tradução, o que mantém o uso de CPU e
RAM previsível.

```bash
# Cada processo carrega somente a capacidade necessária.
./bin/nude-detector serve --models whisper-base
./bin/nude-detector serve --models pp-ocrv6-small
./bin/nude-detector serve --models opus-mt-en-romance
./bin/nude-detector serve --models m2m100-418m
```

Consulte o catálogo antes de iniciar uma capacidade:

```bash
curl -sS http://localhost:8080/v1/models
curl -sS http://localhost:8080/v1/models/whisper-base
```

A resposta informa `state` (`not_installed`, `available` ou `loaded`) e
`enabled`. Um modelo só pode atender inferência se estiver `enabled` e
`loaded` pelo processo atual.

## Endpoints

### `POST /v1/audio/transcriptions`

Recebe `multipart/form-data`. O campo de arquivo obrigatório é `file`; WAV
PCM/float é aceito diretamente e MP3, OGG/Opus, M4A/AAC e FLAC exigem FFmpeg
no host. O tamanho máximo é `MAX_AUDIO_BYTES` e a duração é limitada por
`ASR_MAX_DURATION_SECONDS`.

Campos opcionais:

- `language`: idioma esperado, por exemplo `pt` ou `en`.
- `timestamps`: `true` solicita timestamps quando o modelo os disponibiliza.
- `format`: `json` (padrão) ou `text`.

```bash
curl -sS -X POST http://localhost:8080/v1/audio/transcriptions \
  -F file=@audio.mp3 \
  -F language=pt \
  -F timestamps=true
```

Em JSON, a resposta contém `text`, `language`, `model`, `duration_seconds`,
`processing_time_ms`, `segments` e `metadata`. Não são inventados segmentos
ou timestamps: `segments` pode ser uma lista vazia quando o modelo não expõe
alinhamento confiável.

### `POST /v1/ocr`

Recebe `multipart/form-data` com o campo obrigatório `image`. São aceitas
imagens JPEG, PNG e WebP. `MAX_IMAGE_BYTES` e `OCR_MAX_SIDE` limitam a entrada
antes da inferência.

Campos opcionais:

- `language`: dica de idioma encaminhada ao reconhecedor.
- `return_boxes`: use `true` para receber os blocos e caixas.

```bash
curl -sS -X POST http://localhost:8080/v1/ocr \
  -F image=@documento.png \
  -F language=pt \
  -F return_boxes=true
```

O resultado contém `text`, `model`, `language`, `processing_time_ms`, `blocks`
e `metadata`. Cada caixa usa pixels absolutos da imagem de entrada, em
`{x, y, width, height}`. Sem `return_boxes=true`, `blocks` é uma lista vazia.
PDF, TIFF e `POST /v1/ocr/documents` não estão habilitados nesta versão.

### `POST /v1/translations`

Recebe JSON com exatamente um de `text` ou `texts`, além de
`source_language` e `target_language`. IDs regionais, como `pt-BR`, são
normalizados para o idioma-base suportado. Os limites são
`TRANSLATION_MAX_CHARACTERS` por item e `TRANSLATION_MAX_BATCH` por lote.

```bash
curl -sS -X POST http://localhost:8080/v1/translations \
  -H 'Content-Type: application/json' \
  -d '{"text":"Hello world","source_language":"en","target_language":"pt"}'

curl -sS -X POST http://localhost:8080/v1/translations \
  -H 'Content-Type: application/json' \
  -d '{"texts":["Hello","Good morning"],"source_language":"en","target_language":"pt"}'
```

Uma requisição unitária retorna `source_text`, `translated_text`,
`source_language`, `target_language`, `model`, `processing_time_ms` e
`metadata`. A forma em lote retorna `{ "translations": [...] }` com o mesmo
resultado para cada texto. O campo opcional `model` só é aceito se corresponder
ao modelo de tradução habilitado no processo.

OPUS-MT cobre `en` para `pt`, `es`, `fr` e `it`, ou a direção inversa,
conforme o perfil selecionado. M2M100 cobre pares entre `en`, `pt`, `es`, `fr`
e `it` no perfil atualmente integrado.

### `GET /v1/models` e `GET /v1/models/{id}`

Expõem metadados sem carregar modelos adicionais: ID, versão fixada,
capacidade, engine, quantização, idiomas, memória estimada, licença, estado e
seleção atual. Um ID inexistente retorna `404`.

### Operação existente

`POST /v1/moderation/pornography` e `POST /v1/moderation/export` permanecem
compatíveis com o classificador NSFW. `GET /health`, `GET /ready` e
`GET /metrics` estão disponíveis para operação e observabilidade.

## Erros

Os erros usam o formato:

```json
{"error":{"code":"invalid_request","message":"..."}}
```

Os códigos comuns são `400` para entrada inválida, `413` para limites,
`415` para conteúdo sem suporte, `422` para falha de OCR ou tradução, e `503`
quando a capacidade não foi selecionada no processo. A API não inclui stack
traces nem conteúdo de arquivos ou textos nos logs de resposta.
