# VMVChat

Chat tipo WhatsApp Web para **WhatsApp Business vía CM.com**. Envía y recibe
mensajes (texto, imágenes, stickers, audio, vídeo, documentos y plantillas),
con historial persistente y tiempo real por WebSocket.

- **Backend:** Go + Fiber, SQLite (`modernc.org/sqlite`, sin CGO). Un solo binario.
- **Frontend:** Svelte + Vite, embebido en el binario.
- **Despliegue:** un contenedor. Caddy opcional delante para HTTPS automático.

---

## Arquitectura

```
WhatsApp ──> CM.com ──webhook──> /webhook ──> SQLite
   ^                                 │
   │                                 └─ broadcast WebSocket ─> navegador
   └──── /send, /send-template ──── CM.com API
```

Todo vive en un proceso Go. El frontend se compila en `backend/dist` y se
embebe con `//go:embed`, así que la imagen final solo contiene el binario.

---

## Variables de entorno

| Variable | Obligatoria | Descripción |
|---|---|---|
| `CM_API_KEY` | sí | Product token de CM.com |
| `CM_SENDER` | sí | Número emisor en formato `+34...` |
| `AUTH_USER` | sí | Usuario de acceso a la app |
| `AUTH_PASSWORD` | sí | Contraseña de acceso |
| `WEBHOOK_TOKEN` | recomendada | Token exigido en la URL del webhook (`?token=...`). Vacío = sin comprobación (solo dev) |
| `DOMAIN` | solo TLS | Dominio para el certificado automático de Caddy |

Copia `.env.example` a `.env` y rellénalo. **`.env` está en `.gitignore`** — no se
sube al repo ni entra en la imagen Docker.

---

## Endpoints

| Método | Ruta | Protección | Uso |
|---|---|---|---|
| `GET` | `/health` | pública | Healthcheck (`db.Ping`) |
| `POST` | `/login` `/logout` | pública | Sesión por cookie |
| `GET` | `/webhook` | pública | Validación de CM |
| `POST` | `/webhook?token=` | token | Entrantes + reportes de estado |
| `GET` | `/api/chats` | sesión | Lista de chats con no leídos |
| `GET` | `/api/chats/:phone?before=&limit=` | sesión | Historial paginado |
| `POST` | `/api/chats/:phone/read` | sesión | Marcar como leído |
| `GET` | `/api/search?q=` | sesión | Buscar en mensajes |
| `GET/PUT` | `/api/contacts/:phone` | sesión | Nombre del contacto |
| `GET/POST/DELETE` | `/api/templates` | sesión | CRUD de plantillas |
| `POST` | `/send` `/send-template` | sesión | Enviar |
| `GET` | `/media?url=` | sesión | Proxy de media privada de CM |
| `WS` | `/ws` | sesión (cookie) | Tiempo real |

---

## Despliegue en Portainer

### Opción A — Stack desde el repositorio (recomendada)

1. En Portainer: **Stacks → Add stack → Repository**.
2. URL del repo y ruta del compose (`docker-compose.yml`).
3. En **Environment variables** añade las del bloque de arriba
   (`CM_API_KEY`, `CM_SENDER`, `AUTH_USER`, `AUTH_PASSWORD`, `WEBHOOK_TOKEN`).
   Portainer las inyecta en el `${...}` del compose.
4. **Deploy**. La app queda en el puerto `8080` del host.

### Opción B — Stack pegando el compose

1. **Stacks → Add stack → Web editor**, pega el contenido de `docker-compose.yml`.
2. Rellena las variables en la sección **Environment variables**.
3. **Deploy**.

> El servicio `caddy` está en el perfil `tls` y **no arranca por defecto**. Para
> HTTPS automático, define `DOMAIN` (apuntando al host) y arranca el stack con el
> perfil activo. Si Portainer no expone perfiles de compose, puedes poner un
> reverse proxy/Caddy a nivel de host o quitar el bloque `profiles` del servicio.

### Persistencia

El volumen `vmvchat-data` guarda `chat.db` y los backups diarios
(`/data/backup-YYYY-MM-DD.db`, retención 7 días). Sobrevive a reinicios y
reconstrucciones. **No borres ese volumen** salvo que quieras perder el historial.

### Healthcheck

El compose ya define un healthcheck contra `/health`. En Portainer verás el
contenedor como *healthy*; combinado con `restart: unless-stopped` se recupera
solo si el proceso cae.

---

## Configurar el webhook en CM.com

En el panel de CM, apunta el webhook de WhatsApp a:

```
https://TU-DOMINIO/webhook?token=EL_VALOR_DE_WEBHOOK_TOKEN
```

CM valida con un `GET` (responde 200) y entrega los mensajes con `POST`. Si no
recibe 200 reintenta hasta 7 días; por eso `/webhook` **siempre** responde 200,
incluso ante payloads que no entiende (se registran en el log como
`webhook no parseado`).

---

## Desarrollo local

```bash
# Frontend (genera backend/dist)
cd frontend && npm install && npm run build

# Backend
cd ../backend
CM_API_KEY=... CM_SENDER=+34... AUTH_USER=admin AUTH_PASSWORD=test go run .
# -> http://localhost:8080
```

Para recibir mensajes en local necesitas exponer `/webhook` con una URL pública
(reenvío de puertos de VS Code, túnel, etc.) y configurarla en CM.

Tests del backend:

```bash
cd backend && go test ./...
```

---

## Notas de producción

- **Sesiones:** cookie `httpOnly`, 30 días, almacenadas en SQLite (sobreviven a
  reinicios). No hay caducidad activa ni rate-limit en `/login`: añádelos si
  expones la app a internet abierto.
- **Dedup:** los entrantes se deduplican por huella de contenido, así que un
  reintento del webhook no duplica mensajes.
- **Estados de entrega:** se correlacionan por una `reference` propia que se
  envía a CM. Si los ticks no avanzan a *entregado/leído*, revisa el log por
  `webhook no parseado` con un reporte de estado real y ajusta `parseStatus`.
- **Backup offsite:** el backup actual es local en el mismo volumen. Para
  recuperación ante desastre, copia `/data/backup-*.db` a almacenamiento externo.
