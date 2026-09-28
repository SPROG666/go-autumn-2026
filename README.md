# TripGo — лабораторная работа 1

 - Сервис создаёт, хранит и завершает поездки
 - Данные находятся в PostgreSQL
 - Реализованы ЛР1 и оба бонуса

## Запускаем сервис

Нужны:
 1. Go 1.27.1, make, работающий Docker и `tripgoctl`
 2. Для проверки ручек нужен curl, для запуска сервиса в Docker — Python 3
 3. Все команды выполняем из корня репозитория

### Скачиваем Go-зависимости

```bash
go version # Ожидаем Go 1.27.1
go mod download
```

### Поднимаем окружение и сервис

При первом запуске создаём кластер:

```bash
tripgoctl cluster start
```

Поднимаем PostgreSQL, применяем миграции и запускаем сервис:

```bash
tripgoctl environment start && make migrate && make run
```

Makefile сам читает `.env.example` и `.env`, созданный `tripgoctl`.
Адрес БД можно посмотреть через `tripgoctl connect`.

## Проверяем ручки

Примеры используют порт 8080;

### Проверяем сервис

```bash
curl -i localhost:8080/health
curl -i localhost:8080/ready
```

Обе ручки должны вернуть `200` и `{"status":"ok"}`.

### Создаём поездку

```bash
curl -i -X POST localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",
       "driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5",
       "start_point":{"latitude":59.9398,"longitude":30.3146},
       "end_point":{"latitude":59.9290,"longitude":30.3626},
       "price":1450}'
```

Ожидается `201`, поездка в статусе `active` и заголовок `Location`.
Цена — в целых рублях. Повторяем эту команду: пока поездка активна,
ожидается `409 driver_busy`.

Копируем `id` из успешного ответа:

```bash
TRIP_ID='<id созданной поездки>'
```

### Получаем поездку

```bash
curl -i localhost:8080/api/v1/trips/$TRIP_ID
```

Ожидается `200` и созданная поездка.

### Завершаем поездку

```bash
curl -i -X POST localhost:8080/api/v1/trips/$TRIP_ID/finish
```

Ожидается `200`, статус `completed` и время `finished_at`.
Повторяем команду: ожидается `409 trip_completed`, время завершения не меняется.

### Проверяем ошибки

```bash
# Некорректный UUID — 400 invalid_request
curl -i localhost:8080/api/v1/trips/not-a-uuid

# Нет обязательных полей — 400 invalid_request
curl -i -X POST localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' -d '{}'
```

Для корректного UUID отсутствующей поездки возвращается `404 trip_not_found`.
Ошибки поездок имеют формат `application/problem+json` с полем `code`.

### Проверяем доступность БД

Не останавливая сервис, выполняем:

```bash
tripgoctl environment stop
curl -i localhost:8080/health
curl -i localhost:8080/ready
```

`/health` должен остаться `200`, а `/ready` — вернуть `503`.
Восстанавливаем PostgreSQL:

```bash
tripgoctl environment start
curl -i localhost:8080/ready
```

Теперь `/ready` снова должен вернуть `200`.

## Idempotency-Key

После завершения поездки из примера создаём новую с ключом:

```bash
curl -i -X POST localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: 7f1c2a54-6d0e-4a3b-9f21-0c8e5b3a77d1' \
  -d '{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",
       "driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5",
       "start_point":{"latitude":59.9398,"longitude":30.3146},
       "end_point":{"latitude":59.9290,"longitude":30.3626},
       "price":1450}'
```

Первый запрос с новым ключом вернёт `201`. Повторяем команду без изменений —
получаем `200` и тот же id. Меняем цену, сохранив ключ, — получаем
`409 idempotency_conflict`. Для новой операции используем новый UUID ключа.
Без заголовка выполняется обычное создание

Ключ действует **24 часа** (`IDEMPOTENCY_TTL=24h`). Повторы срок не продлевают.
После истечения ключ можно использовать заново. Просроченные записи очищаются
при запросах с ключом. Актуальное состояние поездки получаем через `GET`. Для идемпотентности - таблица ключей, храним данные запроса и `trip_id`, её создаёт миграция `00003_create_trip_idempotency.sql`

Ключ, поездка и история сохраняются в одной транзакции. Блокировка записи ключа
защищает от одновременных повторов. При ошибке изменения откатываются

## Запуск в Docker

Сначала поднимаем окружение через `tripgoctl`, как описано выше.
Если сервис уже запущен через `make run`, останавливаем его, чтобы освободить порт.

```bash
make docker-build
make migrate && make docker-run
```

Проверяем `/health` и `/ready` теми же командами curl. Остановка — Ctrl+C.

[Dockerfile](Dockerfile) собирает приложение в образе с Go, затем копирует
бинарник в пустой `scratch`. В итоговом образе нет исходников и инструментов
сборки. Приложение работает без root, от пользователя `65532:65532`.
Размер для `linux/arm64` — около **4,7 МБ**, точное значение после сборки:

```bash
docker image inspect trip-service:lab01 --format '{{.Size}} bytes'
```

[run-container.py](deploy/run-container.py) подключает контейнер к PostgreSQL
из `tripgoctl`: заменяет localhost в адресе БД на адрес внутри Docker-сети.
Файл `.env` не изменяется

## Настройки

Все переменные обязательны. Примеры находятся в [.env.example](.env.example),
реальный `DATABASE_URL` берётся из `.env`, созданного `tripgoctl`.

| Переменная | Пример | Назначение |
|---|---|---|
| HTTP_ADDR | :8080 | Адрес сервера |
| LOG_LEVEL | info | Уровень логов |
| SHUTDOWN_TIMEOUT | 10s | Время на остановку сервиса |
| HTTP_READ_TIMEOUT | 5s | Чтение запроса |
| HTTP_READ_HEADER_TIMEOUT | 2s | Чтение заголовков |
| HTTP_WRITE_TIMEOUT | 10s | Запись ответа |
| HTTP_IDLE_TIMEOUT | 60s | Ожидание следующего запроса |
| DATABASE_URL | Из .env | Подключение к PostgreSQL |
| DATABASE_MAX_CONNS | 10 | Максимум соединений |
| DATABASE_MIN_CONNS | 2 | Минимум соединений |
| DATABASE_MAX_CONN_LIFETIME | 30m | Время жизни соединения |
| DATABASE_CONNECT_TIMEOUT | 5s | Таймаут подключения |
| DATABASE_QUERY_TIMEOUT | 3s | Таймаут запроса и транзакции |
| IDEMPOTENCY_TTL | 24h | Срок действия ключа |

## Основные решения

- HTTP-сервер — `net/http` и `chi`. Типы API генерируются из OpenAPI
- Работа с БД — `pgxpool`, SQL строится через `squirrel` с параметрами
- Менеджер `Do(ctx, fn)` передаёт транзакцию через контекст. При успехе делает
  commit, при ошибке или панике — rollback. Вложенный `Do` использует ту же транзакцию
- Уровень изоляции — **Read Committed**: бизнес-правила защищены индексом
  и блокировкой строки, поэтому более строгий уровень здесь не нужен
- Уникальный индекс запрещает две активные поездки одному водителю
- Завершение читает поездку через `SELECT FOR UPDATE`. Второй запрос ждёт
  первый и затем видит завершённый статус
- Поездка и история статусов меняются атомарно: ошибка истории откатывает операцию
- По SIGINT/SIGTERM сервер ждёт активные запросы и закрывает пул БД.
  Если `SHUTDOWN_TIMEOUT` истёк, завершение принудительное

## Команды и проверки

| Команда | Что делает |
|---|---|
| `make generate` | Генерирует `api/api.gen.go` |
| `make build` | Собирает `bin/trip-service` |
| `make test` | Выполняет `go test -race ./...` |
| `make vet` | Проверяет код через `go vet` |
| `make migrate` | Применяет миграции |
| `make migrate-status` | Показывает состояние миграций |
| `make migrate-down` | Откатывает последнюю миграцию с удалением её таблицы и данных |

Команды миграций тоже получают настройки из Makefile — отдельная загрузка env не нужна.

## Останавливаем сервис

Останавливаем сервис через Ctrl+C в терминале запуска. PostgreSQL останавливаем с сохранением данных:

```bash
tripgoctl environment stop
```
