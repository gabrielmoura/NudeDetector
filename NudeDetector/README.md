# Coleção Bruno — NudeDetector

Coleção de testes da API de moderação do NudeDetector. A coleção usa
`http://localhost:8080` como endereço padrão; altere o campo `url` das
requisições para apontar para outro ambiente.

## Fluxo recomendado

1. Execute `GET /health` para verificar o processo.
2. Execute `GET /ready` antes de enviar inferências.
3. Use uma das requisições `Pornography` para classificar uma imagem.
4. Use `Export` para produzir JPEG anotado com boxes e blur opcional.
5. Consulte `GET /metrics` para observar a operação.

O processamento em lote está disponível no binário CLI:

```bash
./bin/nude-detector scan --input "$HOME/.sx" --output-dir ./scan-output --recursive
```

Para somente JSON no stdout, omita `--output-dir`. Para salvar JSONL em arquivo:

```bash
./bin/nude-detector scan --input "$HOME/.sx" \
  --json-output ./scan-results.jsonl
```

O comando sempre escreve um resumo final com `files`, `processed`, `skipped` e
`failed`, mesmo quando não existem arquivos compatíveis.

## Convenções

- Os arquivos são numerados por fluxo e possuem nomes descritivos.
- Imagens de exemplo devem ser colocadas em `fixtures/`; nenhum caminho local
  específico de desenvolvedor é incluído na coleção.
- Não envie credenciais ou imagens sensíveis para ambientes não controlados.
- A classificação semântica usa exclusivamente a classe `porn`; o detector de
  exportação fornece evidência visual complementar.
- Envie `details=true` no multipart ou `"details": true` no JSON para incluir
  as probabilidades semânticas e as detecções NudeNet. Sem esse campo, a
  resposta é compacta.

## Códigos de erro esperados

As respostas de erro usam o formato:

```json
{
  "error": {
    "code": "invalid_image",
    "message": "unable to decode image"
  }
}
```

Consulte a documentação de cada requisição para seus parâmetros e limites.
