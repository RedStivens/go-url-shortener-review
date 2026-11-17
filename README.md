# Сервис сокращения URL

Небольшой HTTP-сервис на Go, который хранит ссылки в памяти процесса. Пользователь определяется по cookie `user_id`, подписанной с помощью HMAC-SHA256.

## Запуск

Требуется Go 1.22 или новее.

```bash
go run ./cmd/shortener
```

Сервис будет доступен по адресу `http://localhost:8080`. После перезапуска созданные ссылки удаляются, так как база данных не используется.

## Проверка

```bash
go test ./...
```

## API

Создать короткую ссылку:

```bash
curl -i -c cookies.txt \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/some/long/path"}' \
  http://localhost:8080/api/shorten
```

Открыть короткую ссылку (идентификатор нужно взять из поля `result` предыдущего ответа):

```bash
curl -i http://localhost:8080/000001
```

Получить ссылки текущего пользователя:

```bash
curl -i -b cookies.txt http://localhost:8080/api/user/urls
```

### Ответы

- `POST /api/shorten` — `201 Created` и JSON с полем `result`;
- `GET /{id}` — `307 Temporary Redirect`, либо `404 Not Found`;
- `GET /api/user/urls` — `200 OK` и JSON-массив, либо `204 No Content`;
- некорректная по формату пользовательская cookie при запросе списка — `401 Unauthorized`.
