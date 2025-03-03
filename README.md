# Merch

Merch это сервис внутреннего магазина мерча компании, где сотрудники могут приобретать товары за монеты. Каждому новому сотруднику выделяется 1000 монет, которые можно использовать для покупки товаров. Кроме того, монеты можно передавать другим сотрудникам в знак благодарности или как подарок.

## Чеклист

* [x] Язык программирования: Go.
* [x] База данных: PostgreSQL.
* [x] Соответствие [заданной OpenAPI спецификации](api/merch/openapi.yaml).
* [x] Авторизация с помощью JWT токенов.
* [ ] Покрытие unit-тестами.

  (Но примеры моих тестов из другого проекта можно посмотреть [тут](https://github.com/k11v/brick/blob/a66ea5b40007d57aa206d2ed698f06b93ae4e91e/internal/buildtask/buildtaskpg/database_test.go).)

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
   go run ./cmd/setup -app
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

## Запуск E2E-тестов

1. Если необходимо, остановите сервер вместе с зависимостями и удалите все данные.

   E2E-тесты не являются идемпотентными, поэтому перед повторным тестированием старых данных быть не должно.

   ```sh
   docker compose down -v
   ```

2. Запустите сервер и его зависимости.

   ```sh
   docker compose up -d
   ```

3. Запустите E2E-тесты.

   APPTEST_URL позволяет указать адрес сервиса, который необходимо протестировать.

   APPTEST_E2E отключает автоматический пропуск E2E-тестов.

   -count=1 отключает автоматическое кэширование тестов, с которым E2E-тесты не запускались бы повторно из-за отсутствия изменений в исходном коде.

   ```sh
   export APPTEST_URL="http://127.0.0.1:8080"
   export APPTEST_E2E=1
   go test -count=1 -v ./tests/e2e/...
   ```

## Запуск load-тестов

1. Если необходимо, остановите сервер вместе с зависимостями и удалите все данные.

   Load-тесты не являются идемпотентными, поэтому перед повторным тестированием старых данных быть не должно.

   ```sh
   docker compose --profile test down -v
   ```

2. Запустите сервер и его зависимости вместе с профилем test.

   Профиль test дополнительно запустит команду `setup -apptest`,
   которая заполнит базу данных тестовыми данными и создаст файлы
   с тестовыми пользователями и токенам аутентификации.

   Токены аутентификации живут столько же, сколько и обычно (1 час),
   поэтому нагрузочное тестирование не следует откладывать на потом.

   ```sh
   docker compose --profile test up -d
   ```

3. Скопируйте файлы с тестовыми пользователями и токенам аутентификации.

   ```sh
   mkdir -p .app/apptest
   docker compose cp server:/user/app/apptest/user.json .app/apptest
   docker compose cp server:/user/app/apptest/auth_token.json .app/apptest
   ```

4. Запустите нагрузочное тестирование, указав пути к скопированным файлам.

   Пути к файлам должны быть указаны абсолютно.

   APPTEST_URL позволяет указать адрес сервиса, который необходимо протестировать.

   ```sh
   export APPTEST_URL="http://127.0.0.1:8080"
   export APPTEST_USER_FILE="$PWD/.app/apptest/user.json"
   export APPTEST_AUTH_TOKEN_FILE="$PWD/.app/apptest/auth_token.json"
   k6 run ./tests/load/server.js
   ```

   Полученные результаты можно сравнить с результатми, которые были получены ранее.

   ```
            /\      Grafana   /‾‾/
       /\  /  \     |\  __   /  /
      /  \/    \    | |/ /  /   ‾‾\
     /          \   |   (  |  (‾)  |
    / __________ \  |_|\_\  \_____/

        execution: local
           script: ./tests/load/server.js
           output: -

        scenarios: (100.00%) 1 scenario, 30 max VUs, 5m30s max duration (incl. graceful stop):
                 * default: Up to 30 looping VUs for 5m0s over 4 stages (gracefulRampDown: 30s, gracefulStop: 30s)


        ✓ 200 or 400 and not enough coin
        ✓ 200

        checks.........................: 100.00% 116469 out of 116469
        data_received..................: 23 MB   76 kB/s
        data_sent......................: 48 MB   159 kB/s
        http_req_blocked...............: avg=13.5µs  min=2µs     med=8µs     max=9.58ms   p(90)=26µs    p(95)=36µs
        http_req_connecting............: avg=290ns   min=0s      med=0s      max=7.85ms   p(90)=0s      p(95)=0s
      ✓ http_req_duration..............: avg=20.7ms  min=2.47ms  med=14.94ms max=442.86ms p(90)=43.48ms p(95)=50.82ms
          { expected_response:true }...: avg=20.7ms  min=2.47ms  med=14.94ms max=442.86ms p(90)=43.48ms p(95)=50.82ms
      ✓ http_req_failed................: 0.00%   0 out of 116469
        http_req_receiving.............: avg=97.08µs min=15µs    med=74µs    max=19.84ms  p(90)=144µs   p(95)=240µs
        http_req_sending...............: avg=68.45µs min=8µs     med=28µs    max=102.92ms p(90)=103µs   p(95)=180µs
        http_req_tls_handshaking.......: avg=0s      min=0s      med=0s      max=0s       p(90)=0s      p(95)=0s
        http_req_waiting...............: avg=20.53ms min=2.39ms  med=14.78ms max=442.8ms  p(90)=43.3ms  p(95)=50.63ms
        http_reqs......................: 116469  388.179964/s
        iteration_duration.............: avg=31.89ms min=12.89ms med=26.18ms max=453.26ms p(90)=54.72ms p(95)=62.06ms
        iterations.....................: 116469  388.179964/s
        vus............................: 19      min=0                max=30
        vus_max........................: 30      min=30               max=30


   running (5m00.0s), 00/30 VUs, 116469 complete and 0 interrupted iterations
   default ✓ [======================================] 00/30 VUs  5m0s
   ```

## Принятые решения

### Реализация HTTP-сервера с помощью [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen)

oapi-codegen это инструмент командной строки и библиотека для преобразования спецификаций OpenAPI в код на языке Go, будь то реализация сервера или клиента.

Данный инструмент был выбран из-за того, что в задании была дана готовая OpenAPI спецификация, которую нельзя было менять и для которой необходимо было реализовать HTTP-сервер. oapi-codegen ускорил разработку слоя представления сервера и позволил реализации сервера максимально соответствовать заданной спецификации. Кроме того, инструмент ускорил написание E2E-тестов за счет генерации API-клиента.

### Хеширование паролей с помощью [Argon2id](https://pkg.go.dev/golang.org/x/crypto/argon2)

Argon2id это версия алгоритма Argon2, победившего в конкурсе Password Hashing Competition 2015 года, и она предназначена для надежного хеширования паролей, обеспечивая защиту от различных атак.

Данный алгоритм был выбран по [рекомендации OWASP](https://owasp.deteact.com/cheat/cheatsheets/Password_Storage_Cheat_Sheet.html). Хеширование паролей было реализовано только с одной внешней зависимостью на [golang.org/x/crypto/argon2](https://pkg.go.dev/golang.org/x/crypto/argon2).

### Использование UUID для первичных ключей в базе данных

UUID в качестве первичных ключей имеют множество преимуществ, например, его никогда не нужно изменять, его может легко сгенерировать и сервер, и клиент, он позволяет безболезненно сливать несколько таблиц в одну. Все это дает гибкость в использовании базы данных по сравнению с альтерантивами (столбец id типа serial, столбец username типа text и другие).

### Хранение переводов, пополнения и трат монет в одной таблице

Таблица transactions создается с помощью следующего запроса:

```sql
CREATE TABLE IF NOT EXISTS transactions (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    from_user_id uuid,
    to_user_id uuid,
    amount integer NOT NULL,
    PRIMARY KEY (id),
    FOREIGN KEY (from_user_id) REFERENCES users (id),
    FOREIGN KEY (to_user_id) REFERENCES users (id)
);
```

Она является источником правды для всех операций с монетами пользователя. Если получить все строки таблицы, где ID пользователя появляется в from_user_id или to_user_id, а затем вычесть или сложить соответствующие строки, то можно получить актуальное количество монет на балансе.

Если пользователь получает монеты извне, то в таблицу вставляется строка (\<id>, NULL, \<user-id>, \<amount>). Пока что такая строка вставляется только, когда создается новый пользователь.

Если пользователь тратит монеты, то добавляется строка (\<id>, \<user-id>, NULL, \<amount>). Такие строки добавляются, когда пользователь покупает товары в магазине.

Если пользователь передает монеты другому пользователю, то добавляется строка (\<id>, \<user-id>, \<other-id>, \<amount>).

Данный подход позволяет учитывать все операции пользователя с монетами. В реальной системе также пригодился бы столбец created_at.

### Хранение баланса как столбца таблицы пользователей

Тем не менее, баланс пользователя также хранится отдельным столбцом в таблице пользователей:

```sql
CREATE TABLE IF NOT EXISTS users (
    id uuid NOT NULL DEFAULT uuid_generate_v4(),
    username text NOT NULL,
    password_hash text NOT NULL,
    balance integer NOT NULL DEFAULT 0,
    PRIMARY KEY (id)
);
```

Такое решение было принято для ускорения операций с монетами. Без уже посчитанного баланса его нужно было бы вычислять при каждой операции. Сейчас баланс изменяется отдельным запросом в HTTP-обработчике, но можно было бы воспользоваться и триггером.

### Структура проекта

Во время разработки большинство кода находилось в [cmd/server/main.go](cmd/server/main.go) для сохранения гибкости и скорости.
Также обходились стороной преждевременные абстракции, такие как сервисный слой и слой репозитория,
чтобы в первую очередь реализовать основной функционал и написать тот код,
который с наибольшим шансом останется в проекте.
Там где абстракции могли принести пользу пользу, они появлялись.

Операции с базой данных инкапсулировлись в отдельные функции (например, getUserByUsername и createTransaction),
чтобы в будущем их было проще вынести в структуру репозитория при необходимости и желании тестировать сервисный слой отдельно.

Ближе к дедлайну файл main.go был разбит на несколько файлов для упрощения понимания и поддержки.
При наличии большего времени на выполнение задания, из HTTP-обработчиков был бы вынесен сервисный слой в отдельные пакеты.
Далее представлены потенциальные типы и функции данных пакетов:

- Пакет internal/auth:
  - Тип Auth (информация об успешной аутентификации).
  - Тип Authenticator:
    - Метод `AuthenticateWithUsernamePassword(username, password string) (*Auth, error)`.
    - Метод `AuthenticateWithToken(token string) (*Auth, error)`.
  - Тип TokenIssuer:
    - Метод `Issue(userID uuid.UUID) (string, error)`.
  - Тип PasswordHasher:
    - Метод `Hash(password string) (string, error)`.
    - Метод `Verify(password, passwordHash string) error`.
- Пакет internal/purchase:
  - Тип Purchase (информация о купленном товаре).
  - Тип Purchaser:
    - Метод `Purchase(userID, itemID uuid.UUID) (*Purchase, error)`.
- Пакет internal/transfer:
  - Тип Transfer (информация о переводе).
  - Тип Transferer:
    - Метод `Transfer(dstUserID, srcUserID uuid.UUID, amount int) (*Transfer, error)`.
- Пакет internal/info:
  - Тип Info (информация с балансом, историей баланса и инвентарем для /api/info).
  - Тип Getter:
    - Метод `Get(userID uuid.UUID) (*Info, error)`.
