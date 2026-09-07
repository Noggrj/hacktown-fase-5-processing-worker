# fiapx-processing-worker

Serviço que efetivamente roda o `ffmpeg` — extrai um frame por segundo do
vídeo e zipa o resultado. É o único dos 4 serviços do FIAP X (Hackathon
SOAT, Fase 5) sem API HTTP de negócio: só consome `video.uploaded` do
Kafka e publica `video.processed`/`video.failed`.

Contratos de evento em [`fiapx-events`](https://github.com/noggrj/hacktown-fase-5-events).
Quem consome os resultados é o
[`fiapx-video-service`](https://github.com/noggrj/hacktown-fase-5-video-service)
(dono da tabela `videos`) e o
[`fiapx-notification-service`](https://github.com/noggrj/hacktown-fase-5-notification-service)
(e-mail em caso de erro).
[Documentação da arquitetura completa](https://github.com/noggrj/hacktown-fase-5-infra/blob/main/docs/ARQUITETURA.md).

## Por que "sem banco próprio"?

Diferente dos outros 3 serviços, este não tem PostgreSQL. A única
persistência que precisaria seria uma tabela `processed_events` para
idempotência — em vez de provisionar um 3º RDS só pra isso, a
idempotência usa **Redis com SETNX+TTL** (`internal/platform/idempotency/redis.go`),
reaproveitando o mesmo Redis que o `fiapx-video-service` já usa como
cache. Uma redelivery rara do Kafka além da janela do TTL reprocessa o
vídeo uma vez a mais — desperdício de CPU, não um bug de correção, já que
os dois consumidores downstream (`video-service`, `notification-service`)
são eles mesmos idempotentes.

## Arquitetura interna

```
cmd/api/main.go                          → wiring (Redis, S3, Kafka), graceful shutdown
internal/processing/usecase/ports.go     → interfaces Downloader/Uploader/FrameExtractor/Publisher
internal/processing/usecase/             → ProcessVideoUseCase (orquestra download→ffmpeg→upload→publish)
internal/processing/ffmpeg/runner.go     → shell-out real pro binário ffmpeg + zip
internal/processing/ffmpeg/zip.go        → empacotamento zip (testável sem ffmpeg)
internal/saga/                           → Consumer (video.uploaded) e Publisher (video.processed/failed) via Kafka
internal/platform/                       → config, logging, health, metrics, cache, storage, messaging, idempotency
```

### Duas categorias de falha, tratadas diferente de propósito

- **Falha de infraestrutura** (S3 fora do ar, publish no Kafka falhando):
  `ProcessVideoUseCase.Execute` retorna erro → o consumer manda a
  mensagem pro tópico `.dlq` em vez de derrubar silenciosamente (ver
  `fiapx-events/transport/kafka`).
- **Falha de negócio** (vídeo corrompido, `ffmpeg` retorna erro): não é
  erro de transporte — vira um evento `video.failed` de verdade, e
  `Execute` retorna `nil` pra a mensagem ser confirmada normalmente. Do
  contrário o Kafka reentregaria pra sempre a mesma mensagem "válida mas
  que sempre falha o processamento".

## Testes

```bash
go test ./...
go test -coverprofile=coverage.out \
  ./internal/processing/usecase/... ./internal/processing/ffmpeg/... \
  ./internal/platform/config/... ./internal/platform/health/... ./internal/platform/metrics/...
go tool cover -func=coverage.out | tail -1
```

`internal/processing/ffmpeg/runner_integration_test.go` gera um vídeo de
teste sintético (`ffmpeg -f lavfi -i testsrc`) e roda o pipeline real —
mas faz `t.Skip` se o binário `ffmpeg` não estiver no `PATH`, então nunca
trava `go test` local. O job de CI instala `ffmpeg` explicitamente, então
esse teste roda de verdade lá.

Mesmo raciocínio de gate de cobertura dos outros repos: `cmd/api` e os
wrappers de infra (`platform/{cache,storage,messaging,idempotency}`,
`internal/saga`) ficam fora do gate.

## Rodando localmente

```bash
cp .env.example .env
go run ./cmd/api
```

Sem Redis/S3/Kafka configurados, o processo sobe (endpoints `/health`,
`/ready`, `/metrics` respondem) mas o consumer não inicia — precisa dos
três pra processar vídeos de verdade.

## Nota sobre o módulo `fiapx-events`

`go.mod` depende de `github.com/noggrj/hacktown-fase-5-events@v1.0.0`, a
tag real publicada no GitHub — sem `replace` local. `docker build .`
funciona standalone.

## Métricas

`/metrics` expõe, além das genéricas de HTTP, as métricas de negócio do
pipeline de processamento (gravadas em
`internal/processing/usecase/process_video.go` — este é o serviço que
sabe de verdade se o `ffmpeg` funcionou, por isso são registradas aqui,
não no `video-service`):

| Métrica | Tipo | O que mede |
|---|---|---|
| `fiapx_videos_processed_total` | contador | Vídeos processados com sucesso |
| `fiapx_videos_failed_total` | contador | Vídeos que falharam (ffmpeg, vídeo corrompido) |
| `fiapx_frames_extracted_total` | contador | Soma de frames extraídos de todos os vídeos |
| `fiapx_video_processing_duration_seconds` | histograma | Duração do `ExtractFrames` (só o ffmpeg, não download/upload) |

Aparecem no dashboard "FIAP X — Serviços" do
[`fiapx-infra`](https://github.com/noggrj/hacktown-fase-5-infra) — local
via `docker-compose` (dá pra gerar dado de verdade subindo um vídeo pelo
frontend e vendo os números mudarem ao vivo), ou em produção via Grafana.

## Deploy

`k8s/base/deployment.yaml` roda 2+ réplicas no mesmo consumer group Kafka
— é isso que atende o requisito de "processar mais de um vídeo ao mesmo
tempo" (cada réplica pega um subconjunto das partições do tópico
`video.uploaded`). HPA escala por CPU (o `ffmpeg` é o único componente do
sistema com uso real de CPU). Sem `JWT_SECRET`: este serviço não expõe
nenhuma rota de negócio, só `/health`/`/ready`/`/metrics`.

### CI/CD — job `deploy`

Pressupõe que a AWS já foi provisionada (`terraform apply` no
[`fiapx-infra`](https://github.com/noggrj/hacktown-fase-5-infra)). Só
roda com disparo manual — nunca em push/PR.

Faz: build + push pro ECR → aplica `k8s/base/` (substituindo o nome real
do bucket no `configmap.yaml` antes) → atualiza a imagem do Deployment e
espera o rollout. Sem Secret — este serviço não tem banco nem JWT, e as
credenciais de S3 em produção vêm da role do node EKS, nunca de env var.

**Secrets/Variables do repositório**:

| Secret / Variable | O que é |
|---|---|
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN` (secrets) | Credenciais temporárias da conta AWS Academy `voclabs` |
| `S3_BUCKET_NAME` (**variable**) | Nome real do bucket (`terraform output videos_bucket_name`) |
