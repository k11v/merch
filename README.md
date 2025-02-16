# Merch

Merch это сервис внутреннего магазина мерча компании, где сотрудники могут приобретать товары за монеты. Каждому новому сотруднику выделяется 1000 монет, которые можно использовать для покупки товаров. Кроме того, монеты можно передавать другим сотрудникам в знак благодарности или как подарок.

## Чеклист

* [x] Язык программирования: Go.
* [x] База данных: PostgreSQL.
* [x] Соответствие [заданной OpenAPI спецификации](api/merch/openapi.yaml).
* [x] Авторизация с помощью JWT токенов.
* [ ] Покрытие unit-тестами.
* [x] Покрытие E2E-тестами.
* [ ] Проведенное нагрузочное тестирование.
* [x] Настроенный golangci-lint.
* [x] Настроенный Docker и Docker Compose.

## Установка и запуск

### Вручную

1. Задайте переменные окружения.

   Из внешних зависимостей необходимо будет поднять PostgreSQL сервер и поместить строку подключения (connection string) в переменную APP_POSTGRES_URL.

   Ключи проверки и подписи JWT являются публичным и приватным ключами ED25519. Если указанные файлы ключей не существуют, они будут автоматически сгенерированы программой setup на следующем шаге.

   ```sh
   export APP_HOST="127.0.0.1"
   export APP_PORT="8080"
   export APP_POSTGRES_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
   export APP_JWT_VERIFICATION_KEY_FILE=".app/jwt-data/public.pem"
   export APP_JWT_SIGNATURE_KEY_FILE=".app/jwt-data/private.pem"
   ```

2. Настройте окружение сервера.

   Программа setup выполнит [миграцию](internal/app/migrationdata) базы данных и сгенерирует ключи проверки и подписи JWT, если необходимо. Она является идемпотентной.

   ```sh
   go run ./cmd/setup
   ```

3. Запустите сервер.

   Сервис будет доступен по адресу http://127.0.0.1:8080.

   ```sh
   go run ./cmd/server
   ```

### Docker Compose

1. Запустите сервер и его зависимости.

   Во время запуска программа setup также выполнит миграцию базы данных и сгенерирует ключи проверки и подписи JWT.

   Сервис будет доступен по адресу http://127.0.0.1:8080.

   ```sh
   docker compose up -d
   ```

## Принятые решения

### Реализация HTTP-сервера с помощью [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)

oapi-codegen это инструмент командной строки и библиотека для преобразования спецификаций OpenAPI в код на языке Go, будь то реализация сервера или клиента.

Данный инструмент был выбран из-за того, что в задании была дана готовая OpenAPI спецификация, которую нельзя было менять и для которой необходимо было реализовать HTTP-сервер. oapi-codegen ускорил разработку слоя представления сервера и позволил реализации сервера максимально соответствовать заданной спецификации. Кроме того, инструмент ускорил написание E2E-тестов за счет генерации API-клиента.
