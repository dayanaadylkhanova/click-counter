````markdown
# Click Counter — HTTP-сервис с поминутной статистикой кликов

## 1. Что делает сервис

Простой сервис, который считает клики по баннерам и возвращает агрегированную статистику по минутам.

**Основные эндпоинты:**
1. `GET /counter/{bannerID}` — засчитывает клик, возвращает `204 No Content`.
2. `POST /stats/{bannerID}` — возвращает JSON-статистику за диапазон `[from, to)` (в UTC).

---

## 2. Переменные окружения

| Переменная | По умолчанию | Описание |
|-------------|---------------|----------|
| `LISTEN_ADDR` | `:3000` | Адрес HTTP-сервера |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DATABASE_URL` | `postgres://postgres:postgres@db:5432/clicks?sslmode=disable` | Подключение к PostgreSQL |
| `FLUSH_EVERY` | `1s` | Интервал сброса данных в БД |
| `SHARDS` | `64` | Количество шардов в памяти |
| `READ_MAX_RANGE_DAYS` | `90` | Максимальная длина интервала `/stats` |
| `SHUTDOWN_WAIT` | `5s` | Таймаут graceful shutdown |
| `MAX_CPU` | `0` | GOMAXPROCS (0 = авто) |

---

## 3. Запуск через Docker Compose

```bash
docker compose -f dev/docker-compose.yml up -d --build
````

Проверить состояние контейнеров:

```bash
docker ps | grep clicks-
```

Проверить, что API живо:

```bash
curl -i http://localhost:3000/healthz
# → HTTP/1.1 204 No Content
```

Остановить сервисы:

```bash
docker compose -f dev/docker-compose.yml down -v
```

---

## 4. Альтернатива: локальный запуск

```bash
docker compose -f dev/docker-compose.yml up -d db

LISTEN_ADDR=:3000 LOG_LEVEL=debug \
DATABASE_URL="postgres://postgres:postgres@localhost:5432/clicks?sslmode=disable" \
go run ./cmd/clicks-api
```

---

## 5. Проверка API

**1. Сделать клики**

```bash
curl -i http://localhost:3000/counter/1
curl -i http://localhost:3000/counter/1
```

**2. Получить статистику**

```bash
FROM=$(date -u -v-1M +"%Y-%m-%dT%H:%M:00Z")
TO=$(date -u +"%Y-%m-%dT%H:%M:00Z")
echo "$FROM -> $TO"

curl -s -X POST http://localhost:3000/stats/1 \
  -H 'Content-Type: application/json' \
  -d "{\"from\":\"$FROM\",\"to\":\"$TO\"}" | jq
```

Ожидаемый ответ:

```json
{
  "stats": [
    {"ts":"2025-10-19T00:29:00Z","v":2}
  ]
}
```

---

## 6. Нагрузочный тест (опционально)

Установить [hey](https://github.com/rakyll/hey):

```bash
brew install hey
```

Прогнать:

```bash
# около 1000 rps
hey -z 10s -c 20 -q 50 http://localhost:3000/counter/1
```

Проверить, что данные попали в статистику:

```bash
FROM=$(date -u -v-1M +"%Y-%m-%dT%H:%M:00Z")
TO=$(date -u +"%Y-%m-%dT%H:%M:00Z")
curl -s -X POST http://localhost:3000/stats/1 \
  -H 'Content-Type: application/json' \
  -d "{\"from\":\"$FROM\",\"to\":\"$TO\"}" | jq
```

---

## 7. Команды Makefile

```bash
make dev-up      # собрать и запустить (db + app)
make dev-down    # остановить и удалить контейнеры
make build       # собрать бинарь локально
make tidy        # tidy зависимостей
```

---

## 8. Структура проекта

```
cmd/clicks-api/main.go         # точка входа
internal/app/...               # жизненный цикл приложения
internal/adapter/transport/http# HTTP-сервер (chi)
internal/adapter/store/postgres# PostgreSQL-хранилище
internal/service/...           # агрегатор кликов
internal/entity/...            # DTO-модели
pkg/config, pkg/logger         # конфиг и zap-логгер
dev/docker-compose.yml, .env   # dev-окружение
migrations/001_init.sql        # схема таблицы
```

---

## 9. Частые проблемы

| Ошибка                                       | Решение                                                                   |
| -------------------------------------------- | ------------------------------------------------------------------------- |
| `/app/clicks-api: no such file or directory` | Удалите строку `- ..:/app` из `dev/docker-compose.yml`.                   |
| `port already in use`                        | Измените порты в `dev/docker-compose.yml`.                                |
| `/stats` возвращает `null`                   | Просто нет данных в диапазоне → возьмите окно шире или предыдущую минуту. |
| `/stats` пустой сразу после клика            | Подождите 1 секунду (`FLUSH_EVERY=1s`).                                   |

---

## 10. Краткий тест-чеклист

1. Запустить сервисы

   ```bash
   docker compose -f dev/docker-compose.yml up -d --build
   ```
2. Проверить `/healthz`

   ```bash
   curl -i http://localhost:3000/healthz
   ```
3. Кликнуть пару раз

   ```bash
   curl -i http://localhost:3000/counter/1
   ```
4. Получить статистику

   ```bash
   FROM=$(date -u -v-1M +"%Y-%m-%dT%H:%M:00Z")
   TO=$(date -u +"%Y-%m-%dT%H:%M:00Z")
   curl -s -X POST http://localhost:3000/stats/1 \
     -H 'Content-Type: application/json' \
     -d "{\"from\":\"$FROM\",\"to\":\"$TO\"}" | jq
   ```
5. Убедиться, что ответ содержит значение `v > 0`.

---

Если всё выше работает — сервис полностью функционален и готов к демонстрации.

```
```
