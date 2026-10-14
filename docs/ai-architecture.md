# Arquitetura de modelos de IA

## Auditoria inicial

O binário `nude-detector` usa Cobra para a CLI e `net/http` para a API. O
comando `serve` chama `newRuntime`, que inicializa o ambiente global do ONNX
Runtime, garante o download do classificador NSFW, cria uma sessão ONNX e faz
um warm-up. Quando `DETECTOR_ENABLED=true`, também baixa e cria uma segunda
sessão para NudeNet. A mesma composição é reutilizada pelos comandos locais
existentes.

O processamento e a política de moderação já estão corretamente separados:
`imageprocessor` produz o tensor, `inference` executa o classificador e
`moderation` aplica os limiares. `modelstore` já oferece download temporário,
checksum e renomeação atômica, mas precisa ganhar limites de tamanho e suporte
a artefatos compostos antes de instalar modelos novos.

Não há fila externa, banco de dados ou framework de jobs. As requisições HTTP
usam limites de imagem, timeouts do servidor, um worker de inferência com
backpressure e métricas Prometheus. O Docker é multi-stage, Debian/glibc e
executa como usuário não privilegiado.

## Decisão: seleção explícita no modo serve

O servidor passa a receber uma lista de modelos habilitados por
`--models`/`ENABLED_MODELS`. O padrão de compatibilidade é somente
`nsfw-classifier`; `nudenet-detector`, ASR, OCR e tradução permanecem
desabilitados até que sejam explicitamente selecionados e instalados. O
registro é a fonte única de metadados estáticos e o estado de instalação é
calculado separadamente. Isto evita download, consumo de RAM e inicialização
de bibliotecas nativas para modelos que a instância não vai servir.

No host, os artefatos opcionais são instalados explicitamente com `models
install`. A imagem Docker ativa `MODEL_AUTO_INSTALL=true`: durante `serve`, o
processo resolve somente os manifests dos IDs selecionados, verifica SHA-256 e
instala de forma atômica no volume de modelos antes de carregar qualquer
sessão. Esse fluxo não existe em handlers HTTP nem nos comandos de inferência
da CLI.

O detector de detalhes continua opcional: selecionar `nudenet-detector` sem
`DETECTOR_ENABLED=true` é uma configuração inválida, e selecionar
`nudenet-detector` habilita a sua carga sem mudar o comportamento do
classificador principal. O classificador NSFW só é criado quando
`nsfw-classifier` está selecionado; os seus endpoints retornam indisponível se
ele não fizer parte da instância.

## Organização incremental

1. `internal/aimodel`: catálogo, capacidades, estados e validação da seleção.
2. Composição do runtime: inicializar somente engines e sessões dos modelos
   selecionados; expor `GET /v1/models`.
3. ASR: adapter Sherpa-ONNX isolado do wrapper ONNX Runtime existente e
   modelos Whisper instalados por artefatos fixados. WAV PCM/float já é
   validado; FFmpeg seguro para os codecs adicionais permanece pendente.
4. OCR: detector e reconhecedor PP-OCRv6 como um perfil composto, incluindo
   dicionário e pós-processamento DB/CTC validado contra os contratos ONNX.
5. Tradução: OPUS-MT agora usa o `tokenizer.json` oficial, encoder ONNX e
   decoder autoregressivo real. Os smoke tests opt-in validam `Hello world` →
   `Olá mundo` e a direção inversa. M2M100 também usa seus artefatos ONNX
   oficiais, tokens de idioma e geração autoregressiva; a primeira versão usa
   histórico completo em vez de cache KV para manter o contrato validado.

## Compatibilidade e riscos

Sherpa-ONNX usa CGO e fornece suas próprias bibliotecas nativas. Não deve
compartilhar implicitamente nem substituir a `libonnxruntime` usada pelo
wrapper atual. O adapter ASR precisa de uma matriz de ABI por plataforma e de
testes reais em Linux amd64/arm64, macOS e Windows. FFmpeg será opcional na
imagem de desenvolvimento, mas obrigatório para MP3, OGG/Opus, M4A/AAC e
FLAC; WAV PCM pode ser aceito sem ele.

PP-OCRv6 não é um único modelo: cada perfil requer detector, reconhecedor e
dicionário compatível. As dimensões, normalização, pós-processamento e licença
devem ser extraídos das revisões fixadas antes de codificar o pipeline.

OPUS-MT e M2M100 são seq2seq: uma sessão ONNX isolada não resolve a tradução.
A implementação deve validar tokenização, entradas/saídas de encoder e
decoder, tokens de idioma, parada e cache antes de declarar suporte.

## Estimativas operacionais iniciais

As estimativas abaixo são orçamento de planejamento, não medições. O modelo
NSFW atual e NudeNet são pequenos frente aos novos modelos. Whisper Tiny,
Base e Small em INT8 normalmente exigem respectivamente dezenas, centenas e
várias centenas de MB de artefatos e RAM; PP-OCRv6 precisa de duas sessões por
perfil; OPUS-MT costuma ser adequado para CPU, enquanto M2M100 418M exige um
orçamento de RAM substancial. As métricas de carregamento e RSS devem decidir
os valores documentados finais.

## Estratégia de testes

Testes unitários cobrirão seleção, configuração e estado do registro sem
modelos baixados. Testes de integração opt-in usarão modelos reais já
instalados e nunca baixarão artefatos durante uma requisição. Cada capacidade
terá smoke test de carga e um caso de referência: fala PT/EN e silêncio para
ASR, texto PT e blocos múltiplos para OCR, e EN↔PT para tradução. API e CLI
invocarão os mesmos serviços de aplicação.
