# court-snapshots

Servidor standalone em Go que recebe snapshots de cameras IP via FTP e serve a imagem mais recente via HTTP, alimentando um pipeline de analise por IA.

## Fluxo de funcionamento

```
                        FTP (porta 21)                  HTTP GET
  Cameras Intelbras  ──────────────────>  vsftpd  ──>  Go API  ──>  Nginx (HTTPS)
  (upload periodico)                      (chroot       (serve       (reverse proxy
                                          por user)     latest)      + SSL)
                                              │             │
                                              v             v
                                         /snapshots/    Responde ao
                                         court-{id}/    analyze-court
                                         *.jpg          com a imagem
```

**Passo a passo:**

1. Cada camera faz upload periodico de snapshots via FTP para a VPS
2. O vsftpd recebe os arquivos — cada camera tem credencial propria e fica isolada (chroot) na pasta da sua quadra
3. O pipeline de IA chama `GET /snapshots/:courtId/latest` com autenticacao via API key
4. O servidor Go localiza a imagem `.jpg` mais recente no diretorio da quadra, retorna como `image/jpeg`
5. Apos servir, todos os arquivos da quadra sao deletados em background (evita acumulo em disco)

## Arquitetura

```
court-snapshots/
├── main.go                  # Entrypoint
├── config/
│   └── config.go            # Env vars e validacao
├── server/
│   └── server.go            # HTTP server, rotas e logging middleware
├── handler/
│   └── snapshot.go          # Handler GET /snapshots/:courtId/latest
├── storage/
│   └── disk.go              # Leitura do snapshot mais recente + cleanup
├── auth/
│   └── apikey.go            # Middleware de autenticacao por API key
├── ftpusers/
│   └── manage.sh            # Script para criar/remover users FTP
├── deploy/
│   ├── Dockerfile           # Multi-stage build (Go -> Alpine)
│   ├── docker-compose.yml   # App + vsftpd + Nginx
│   ├── nginx.conf           # Reverse proxy com HTTPS
│   ├── vsftpd.conf          # Configuracao do servidor FTP
│   └── certbot-renew.sh     # Renovacao automatica de certificado SSL
├── .env.example
└── .gitignore
```

## API

### `GET /health`

Health check sem autenticacao. Para monitoramento e uptime checks.

```bash
curl https://snapshots.seudominio.com/health
# {"status":"ok"}
```

### `GET /snapshots/{courtId}/latest`

Retorna o snapshot mais recente da quadra especificada.

**Headers obrigatorios:**
```
Authorization: Bearer <API_KEY>
```

**Parametros:**
| Parametro | Tipo | Descricao |
|-----------|------|-----------|
| `courtId` | UUID | Identificador da quadra (formato `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`) |

**Respostas:**

| Status | Quando | Body |
|--------|--------|------|
| `200`  | Imagem retornada com sucesso | `image/jpeg` (binario) |
| `400`  | Court ID com formato invalido | `{"error": "invalid court ID"}` |
| `401`  | API key ausente ou invalida | `{"error": "unauthorized"}` |
| `404`  | Quadra nao encontrada ou sem snapshot | `{"error": "court not found"}` ou `{"error": "no snapshot available"}` |
| `500`  | Erro de I/O | `{"error": "internal error"}` |

**Headers da resposta 200:**
```
Content-Type: image/jpeg
Cache-Control: no-store
X-Snapshot-File: snap_20260307_143300.jpg
```

**Exemplo:**
```bash
curl -H "Authorization: Bearer SUA_API_KEY" \
  https://snapshots.seudominio.com/snapshots/abc12345-1234-5678-9abc-def012345678/latest \
  -o snapshot.jpg
```

## Configuracao

Variaveis de ambiente (ver `.env.example`):

| Variavel | Default | Descricao |
|----------|---------|-----------|
| `PORT` | `8080` | Porta do servidor HTTP |
| `SNAPSHOTS_DIR` | `/snapshots` | Diretorio raiz onde as cameras gravam os snapshots |
| `API_KEY` | — (obrigatorio) | Chave para autenticar requests na API |
| `DELETE_AFTER_SERVE` | `true` | Apagar imagens apos servir (evita acumulo em disco) |
| `VPS_PUBLIC_IP` | — | IP publico da VPS (usado pelo vsftpd no modo passivo) |

## Seguranca

- **Autenticacao** — API key via header `Authorization: Bearer <key>`, comparacao com `crypto/subtle.ConstantTimeCompare` (protege contra timing attacks)
- **FTP isolado** — cada camera tem user Linux proprio, chroot na pasta da quadra, sem shell (`/usr/sbin/nologin`)
- **Validacao de input** — court ID aceita apenas UUID valido via regex
- **Sem acumulo** — imagens deletadas apos servir, disco nunca cresce
- **HTTPS** — Nginx como reverse proxy com certificado Let's Encrypt
- **Firewall** — apenas portas 21 (FTP), 30000-30100 (FTP passivo), 80 (redirect), 443 (HTTPS)

## Caracteristicas tecnicas

- **Go 1.22+** com roteamento nativo (`net/http` ServeMux com path parameters)
- **Zero dependencias externas** — stdlib pura
- **Cleanup assincrono** — arquivos deletados em goroutine apos envio da resposta (nao bloqueia o client)
- **Logging** — middleware registra metodo, path, status code e duracao de cada request
- **Timeouts** — ReadTimeout 10s, WriteTimeout 30s, IdleTimeout 60s
- **Multi-stage Docker build** — imagem final baseada em Alpine (~15MB)

## Deploy

### Pre-requisitos

- VPS com Docker e Docker Compose instalados
- Dominio apontando para o IP da VPS (ex: `snapshots.seudominio.com`)

### Passos

1. Clonar o repositorio na VPS:
   ```bash
   git clone git@github.com:nandokferrari/court-snapshots.git
   cd court-snapshots
   ```

2. Configurar variaveis de ambiente:
   ```bash
   cp .env.example .env
   # editar .env com valores reais
   ```

3. Gerar certificado SSL:
   ```bash
   certbot certonly --standalone -d snapshots.seudominio.com
   ```

4. Subir os containers:
   ```bash
   cd deploy && docker compose up -d
   ```

5. Configurar renovacao automatica do certificado:
   ```bash
   # Adicionar ao crontab
   0 3 * * * /caminho/deploy/certbot-renew.sh
   ```

### Cadastro de nova camera

1. Na VPS, criar user FTP para a quadra:
   ```bash
   ./ftpusers/manage.sh add <court-uuid> <senha>
   ```

2. Na camera Intelbras VIP, configurar FTP:
   - Servidor: IP da VPS
   - Porta: 21
   - Usuario: `cam_XXXXXXXX` (primeiros 8 chars do UUID)
   - Senha: a senha definida no passo anterior
   - Path: `/`

3. No Supabase (tabela `courts`), preencher `camera_snapshot_url`:
   ```
   https://snapshots.seudominio.com/snapshots/<court-uuid>/latest
   ```

4. Pronto — o pipeline de analise ja inclui a quadra no proximo ciclo

### Desativar uma camera

Basta limpar o campo `camera_snapshot_url` no banco. A camera pode continuar mandando FTP sem problema — as imagens serao ignoradas e eventualmente limpas.

## Estrutura de pastas FTP

```
/snapshots/
├── court-{uuid-1}/              <- home do user cam_xxxxxxx1
│   ├── snap_20260307_143200.jpg
│   └── snap_20260307_143300.jpg
├── court-{uuid-2}/              <- home do user cam_xxxxxxx2
│   └── snap_20260307_143200.jpg
```

Cada camera grava na raiz da sua pasta (chroot). O nome do arquivo deve conter timestamp para garantir ordenacao correta — as cameras Intelbras fazem isso nativamente.

## Monitoramento

- **Health check** externo (UptimeRobot ou similar) em `GET /health`
- **Disk usage** — com `DELETE_AFTER_SERVE=true`, disco nunca deve passar de poucos MB
- **Logs** — `docker compose logs -f app` para acompanhar requests em tempo real
- Se uma quadra parar de receber snapshots, o pipeline de IA recebe 404 e registra como erro

## Custos estimados

| Item | Custo |
|------|-------|
| VPS Hetzner CX22 (2vCPU, 2GB RAM, 40GB) | ~4 EUR/mes |
| Dominio (se necessario) | ~10 USD/ano |
| Let's Encrypt | Gratis |
| **Total** | **~5 USD/mes** |
