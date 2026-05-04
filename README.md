# Soundcloud-like backend на Go

Backend MVP с тремя основными модулями:

- авторизация: регистрация, логин, JWT;
- загрузка музыки: multipart upload аудиофайла;
- импорт SoundCloud: скачивание трека или альбома/плейлиста по ссылке через `yt-dlp`;
- прослушивание: metadata API и streaming endpoint с поддержкой `Range`.

Сейчас metadata пользователей и треков хранится в Postgres, а аудиофайлы лежат в MinIO/S3 или локальной папке.

## Запуск через Docker Compose

```powershell
docker compose up --build
```

Сервисы:

- API: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger`
- OpenAPI YAML: `http://localhost:8080/swagger/openapi.yaml`
- Postgres: `localhost:5432`
- MinIO S3 API: `http://localhost:9000`
- MinIO Console: `http://localhost:9001`

Логины для dev-окружения:

- Postgres: `soundcloud` / `soundcloud`, database `soundcloud`
- MinIO: `minioadmin` / `minioadmin`

Backend сам применяет минимальные миграции при старте и создает bucket `tracks`, если его еще нет.

## Запуск локально без Docker для API

Сначала подними Postgres и MinIO через compose:

```powershell
docker compose up postgres minio
```

Потом запусти API на хосте:

```powershell
go run ./cmd/api
```

Для локального запуска используются значения из `.env.example`:

```env
DATABASE_URL=postgres://soundcloud:soundcloud@localhost:5432/soundcloud?sslmode=disable
STORAGE_DRIVER=s3
S3_ENDPOINT=localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=tracks
S3_REGION=us-east-1
S3_USE_SSL=false
```

Если API запущен внутри `docker compose`, S3 endpoint должен быть `minio:9000`. Если API запущен через `go run` на хосте, endpoint должен быть `localhost:9000`.

Для импорта SoundCloud при локальном запуске без Docker на хосте должны быть доступны `yt-dlp` и `ffmpeg`.
Путь к бинарнику `yt-dlp` можно переопределить через `YT_DLP_BINARY`.

## API

### Регистрация

```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "demo@example.com",
  "username": "demo",
  "password": "password123"
}
```

### Логин

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "demo@example.com",
  "password": "password123"
}
```

### Загрузка трека

```powershell
curl.exe -X POST http://localhost:8080/api/v1/tracks `
  -H "Authorization: Bearer <JWT>" `
  -F "title=First Track" `
  -F "artist=Demo Artist" `
  -F "audio=@D:\music\track.mp3"
```

### Импорт трека из SoundCloud

```http
POST /api/v1/tracks/import/soundcloud
Authorization: Bearer <JWT>
Content-Type: application/json

{
  "url": "https://soundcloud.com/artist/track",
  "album_id": ""
}
```

API скачивает аудио, конвертирует его в mp3 и сохраняет в тот же storage, что и обычные загрузки.

### Импорт альбома или плейлиста из SoundCloud

```http
POST /api/v1/albums/import/soundcloud
Authorization: Bearer <JWT>
Content-Type: application/json

{
  "url": "https://soundcloud.com/artist/sets/album"
}
```

API создает альбом и последовательно скачивает его треки.

### Список треков

```http
GET /api/v1/tracks
```

### Прослушивание

```http
GET /api/v1/tracks/{id}/stream
```

Endpoint отдает аудио через `http.ServeContent`, поэтому браузеры и плееры могут запрашивать диапазоны байтов.

Проверка `Content-Type`:

```powershell
curl.exe -I http://localhost:8080/api/v1/tracks/<id>/stream
```

Для MP3 должен быть `Content-Type: audio/mpeg`.

## CD

В репозитории настроен GitHub Actions workflow [`.github/workflows/cd.yml`](.github/workflows/cd.yml).

Он запускается:

- при `push` в `main` или `master`;
- при пуше тега вида `v*`;
- вручную через `workflow_dispatch`.

Workflow собирает Docker-образ из [`Dockerfile`](Dockerfile) и публикует его в GHCR:

- `ghcr.io/<owner>/soundcloud-api:latest` для default branch;
- `ghcr.io/<owner>/soundcloud-api:<branch>`;
- `ghcr.io/<owner>/soundcloud-api:<tag>`;
- `ghcr.io/<owner>/soundcloud-api:sha-<commit>`.

Для веток `main` и `master` workflow также деплоит приложение на удаленный сервер по SSH.

Серверный запуск использует [`docker-compose.prod.yml`](docker-compose.prod.yml), ожидает соседний checkout frontend-репозитория в `../soundcloud-front` и поднимает стек командой:

```sh
docker compose -f docker-compose.prod.yml up -d --build --remove-orphans
```

В production публичный трафик идет через `Caddy`, который сам получает TLS-сертификат и терминирует `https://dropwave.ru` на `80/443`.

Перед первым деплоем в GitHub repository secrets нужно добавить:

- `DEPLOY_HOST`
- `DEPLOY_USERNAME`
- `DEPLOY_PASSWORD`

На сервере workflow автоматически:

- клонирует или обновляет `https://github.com/shoumq/soundcloud` в `/opt/soundcloud`;
- клонирует или обновляет `https://github.com/shoumq/soundcloud-front` в `/opt/soundcloud-front`;
- создает `.env.production` из [`.env.production.example`](.env.production.example), если файла еще нет.

После первого деплоя нужно вручную открыть и заполнить `/opt/soundcloud/.env.production` продовыми значениями.
