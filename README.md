# court-snapshots

Servidor standalone em Go que recebe snapshots de cameras IP via FTP e serve a imagem mais recente via HTTP, alimentando um pipeline de analise por IA.

## Fluxo de funcionamento

```
                        FTP (porta 21)                  HTTP GET
  Cameras / ESP32    ──────────────────>  vsftpd  ──>  Go API
  (upload periodico)                      (virtual       (serve
                                          users +        latest)
                                          per-user
                                          local_root)
                                              │             │
                                              v             v
                                         /snapshots/    Responde com
                                         court-{id}/    a imagem mais
                                         *.jpg          recente
```

**Passo a passo:**

1. Cada camera/ESP faz upload periodico de snapshots via FTP para a VPS
2. O vsftpd recebe os arquivos — cada camera tem credencial propria e o `local_root` por usuario direciona os uploads para `/snapshots/court-{uuid}/`
3. O pipeline de IA chama `GET /snapshots/:courtId/latest` com autenticacao via API key
4. O servidor Go localiza a imagem `.jpg` mais recente no diretorio da quadra, valida que o JPEG esta completo (marcador `FF D9`), e retorna como `image/jpeg`

## Arquitetura

```
court-snapshots/
├── main.go                  # Entrypoint
├── config/
│   └── config.go            # Env vars e validacao
├── server/
│   └── server.go            # HTTP server, rotas e logging middleware
├── handler/
│   └── snapshot.go          # Handlers: latest, list, file, thumbnails
├── storage/
│   └── disk.go              # Leitura de snapshots, validacao JPEG, listagem
├── auth/
│   └── apikey.go            # Middleware de autenticacao por API key
├── ftpusers/
│   └── manage.sh            # Script para criar/remover users FTP
├── deploy/
│   ├── Dockerfile           # Multi-stage build (Go -> Alpine)
│   ├── docker-compose.yml   # App + vsftpd
│   ├── vsftpd.conf          # Configuracao do servidor FTP (virtual users)
│   ├── vsftpd-entrypoint.sh # Wrapper que injeta user_config_dir no config
│   └── user_conf/           # Per-user vsftpd configs (local_root por camera)
├── .env.example
└── .gitignore
```

## API

Todas as rotas (exceto `/health`) exigem autenticacao via header `Authorization: Bearer <API_KEY>` ou query param `?key=<API_KEY>`.

### `GET /health`

Health check sem autenticacao.

```bash
curl http://host:8080/health
# {"status":"ok"}
```

### `GET /snapshots/{courtId}/latest`

Retorna o snapshot **mais recente** da quadra (ordenado pelo nome do arquivo). Imagens com upload incompleto (sem marcador JPEG `FF D9`) sao ignoradas.

```bash
curl -H "Authorization: Bearer <KEY>" http://host:8080/snapshots/<courtId>/latest -o snap.jpg
```

| Status | Quando |
|--------|--------|
| `200`  | Imagem retornada (`image/jpeg`) |
| `400`  | Court ID invalido |
| `401`  | API key ausente ou invalida |
| `404`  | Quadra nao encontrada ou sem snapshot disponivel |

### `GET /snapshots/{courtId}/list`

Lista todos os snapshots da quadra em ordem decrescente (mais recente primeiro), com contagem, tamanho e URL para abrir cada arquivo.

```bash
curl -H "Authorization: Bearer <KEY>" http://host:8080/snapshots/<courtId>/list
```

```json
{
  "court_id": "372a8e8d-...",
  "count": 14,
  "files": [
    {
      "filename": "snap_20260531_120000.jpg",
      "size_bytes": 78528,
      "created_at": "2026-05-31 12:00:00",
      "open_url": "/snapshots/372a8e8d-.../file/snap_20260531_120000.jpg"
    }
  ]
}
```

### `GET /snapshots/{courtId}/file/{filename}`

Serve um arquivo de snapshot especifico como `image/jpeg`.

```bash
curl -H "Authorization: Bearer <KEY>" http://host:8080/snapshots/<courtId>/file/snap_20260531_120000.jpg -o snap.jpg
```

### `GET /snapshots/{courtId}/thumbnails`

Pagina HTML com grid visual das ultimas 100 imagens (mais recente primeiro). Cada thumbnail tem 200px de largura com link para a imagem full size. Util para monitoramento visual rapido.

```
http://host:8080/snapshots/<courtId>/thumbnails?key=<API_KEY>
```

## Configuracao

Variaveis de ambiente (ver `.env.example`):

| Variavel | Default | Descricao |
|----------|---------|-----------|
| `PORT` | `8080` | Porta do servidor HTTP |
| `SNAPSHOTS_DIR` | `/snapshots` | Diretorio raiz dos snapshots |
| `API_KEY` | — (obrigatorio) | Chave para autenticar requests |
| `DELETE_AFTER_SERVE` | `true` | Apagar imagens apos servir via `/latest` |

## Seguranca

- **Autenticacao** — API key via header ou query param, comparacao com `crypto/subtle.ConstantTimeCompare`
- **FTP isolado** — virtual users com `local_root` por usuario, cada camera so acessa o diretorio da sua quadra
- **Validacao de input** — court ID aceita apenas UUID valido via regex
- **Integridade** — imagens incompletas (sem marcador JPEG EOI `FF D9`) nao sao servidas
- **Firewall** — apenas portas 21 (FTP), 30000-30100 (FTP passivo), 8080 (API)

## Caracteristicas tecnicas

- **Go 1.22+** com roteamento nativo (`net/http` ServeMux com path parameters)
- **Zero dependencias externas** — stdlib pura
- **Validacao JPEG** — verifica marcador EOI (`FF D9`) nos ultimos 2 bytes antes de servir, evitando imagens parciais durante upload
- **Logging** — middleware registra metodo, path, status code e duracao de cada request
- **Timeouts** — ReadTimeout 10s, WriteTimeout 30s, IdleTimeout 60s
- **Multi-stage Docker build** — imagem final baseada em Alpine (~15MB)

## Deploy

### Pre-requisitos

- VPS com Docker e Docker Compose instalados

### Passos

1. Clonar o repositorio na VPS:
   ```bash
   git clone git@github.com:nandokferrari/court-snapshots.git /opt/court-snapshots
   cd /opt/court-snapshots
   ```

2. Configurar variaveis de ambiente:
   ```bash
   cp .env.example .env
   # editar .env com valores reais
   ```

3. Subir os containers:
   ```bash
   cd deploy && docker compose up -d
   ```

### Cadastro de nova camera

1. Criar arquivo de configuracao FTP para o usuario em `deploy/user_conf/<ftp_user>`:
   ```
   local_root=/snapshots/court-<court-uuid>
   ```

2. Adicionar o usuario FTP nas variaveis de ambiente do vsftpd no `docker-compose.yml`

3. Recriar o container vsftpd:
   ```bash
   docker compose up -d --force-recreate vsftpd
   ```

4. Configurar a camera/ESP:
   - Host: IP da VPS
   - Porta: 21
   - Usuario e senha conforme configurado
   - Path: `/` (o `local_root` direciona para o diretorio correto)

## Estrutura de pastas FTP

```
/snapshots/
├── court-{uuid-1}/              <- local_root do user cam_xxx
│   ├── snap_20260531_120000.jpg
│   └── snap_20260531_120030.jpg
├── court-{uuid-2}/              <- local_root do user cam_yyy
│   └── snap_20260531_120000.jpg
```

Cada camera grava na raiz do seu FTP (o `local_root` por usuario direciona para a pasta correta). O nome do arquivo deve conter timestamp para garantir ordenacao correta.

## Monitoramento

- **Health check** externo em `GET /health`
- **Thumbnails** — `GET /snapshots/{courtId}/thumbnails?key=...` para visualizar os snapshots recebidos
- **List** — `GET /snapshots/{courtId}/list?key=...` para contagem e detalhes dos arquivos
- **Logs** — `docker compose logs -f app` para acompanhar requests em tempo real
