# Nutrition Service

Сервис профилей питания, расчёта дневных норм и каталога продуктов.

## Структура

- `internal/domain` — модели профиля и продукта.
- `internal/service` — проверка данных, расчёт калорий и БЖУ, правила доступа.
- `internal/repository/postgres` — запросы к PostgreSQL.
- `internal/handler` — JSON API и HTTP-коды.
- `internal/server` — chi-router, JWT middleware, health/readiness/metrics.
- `migrations` — SQL-миграции базы Nutrition.

`FoodService` проверяет входные данные; `FoodRepo` ограничивает видимость и изменение продуктов по `owner_user_id`.
Продукт с `owner_user_id IS NULL` — общий, его можно читать, но нельзя редактировать или удалять через пользовательский API.

## HTTP API

Все маршруты ниже требуют `Authorization: Bearer <JWT>`.

| Метод | Путь | Успех |
| --- | --- | --- |
| POST | `/nutrition/profile` | 200, профиль |
| GET | `/nutrition/profile` | 200, профиль / 404 |
| GET | `/nutrition/targets` | 200, дневные нормы / 404 |
| POST | `/nutrition/foods` | 201, продукт + заголовок Location |
| GET | `/nutrition/foods?limit=20&offset=0` | 200, массив продуктов |
| GET | `/nutrition/foods/{id}` | 200, продукт / 404 |
| PUT | `/nutrition/foods/{id}` | 200, изменённый продукт / 404 |
| DELETE | `/nutrition/foods/{id}` | 204 без тела / 404 |

Параметры каталога: `limit` от 1 до 100 (по умолчанию 20), `offset` неотрицательный (по умолчанию 0).
`PUT` заменяет все пользовательские поля продукта, не является `PATCH`.

### Пример продукта

```json
{
  "name": "Oatmeal",
  "brand": "Example",
  "calories_per_100g": 180,
  "protein_per_100g": 6,
  "fat_per_100g": 4,
  "carbs_per_100g": 30
}
```

Калории и БЖУ задаются **на 100 граммов**, обязательны и допускают до двух знаков после запятой.
ID продукта и идентификатор владельца назначает сервер.
При удалении заполняется `deleted_at`; запись остаётся в базе, но не возвращается публичными GET-запросами.

### Профиль

Поля запроса: `age`, `sex`, `height_cm`, `weight_kg`, `activity_level`, `goal`.
Поле `user_id`, если передано в JSON, намеренно игнорируется для обратной совместимости: пользователь определяется только через JWT.

JSON запросов ограничен 1 МиБ; неизвестные поля (кроме совместимого `user_id` в профиле), лишние JSON-объекты и некорректные типы приводят к `400`, превышение размера — к `413`.
Для операций с продуктами и профилем некорректный UUID отклоняется до обращения к PostgreSQL.

## Локальная разработка

Из корня монорепозитория:

```powershell
docker compose up -d postgres_nutrition
go test -count=1 ./services/nutrition/...
go build ./services/nutrition/...
```

Для запуска сервера из PowerShell установите переменные среды и смените рабочую директорию:

```powershell
$env:DATABASE_URL = "postgres://admin:secret@localhost:5436/nutritiondb?sslmode=disable"
$env:JWT_SECRET = "my-super-secret-key"
$env:HTTP_PORT = "8084"
Set-Location services/nutrition
go run ./cmd/server
```

Важно: `runMigrations` использует путь `file://migrations`, относительный к текущей рабочей директории.
В Docker миграции копируются в `/migrations`; при локальном запуске команда выше должна выполняться из `services/nutrition`.

Интеграционные тесты используют Testcontainers и требуют запущенного Docker Desktop.
Оба HTTP-набора (профиль и продукты) используют один тестовый контейнер внутри `TestNutritionProfileHTTP`.

## Технические границы

- Валидация суммарного БЖУ находится в `FoodService`; новую миграцию для проверки суммы пока не добавляли.
- Расчёт дневных норм использует существующую формулу сервиса; при изменении правил расчёта обновляйте тесты осознанно.
- Общий каталог не предоставляет через пользовательский API создание или изменение системных общих продуктов.
