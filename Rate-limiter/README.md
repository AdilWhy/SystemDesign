## Rate Limiter Implementation

The microservice now includes a rate-limiting mechanism to ensure stability and consistent performance during traffic spikes.

### Rate Limiting Approach

**Algorithm: Token Bucket**
1. У каждого клиента своё ведро: Для каждого клиента (по ID или IP-адресу) создаётся отдельное «ведро».
2. Ведро содержит токены: Каждый токен — это разрешение на один запрос.
3. Токены пополняются с какой-то выбранной скоростью или же `RATE_LIMIT_REFILL_RATE`, но они не могут превысить максимальное количество, т.е `RATE_LIMIT_CAPACITY`
4. Запросы тратят токены: Когда клиент делает запрос, из ведра убирается один жетон.
5. Если ведро пустое — 429 ошибка

Тестил с `RATE_LIMIT_CAPACITY`=2 и `RATE_LIMIT_REFILL_RATE` = 1 

![Ручной тест](Postman_Lb9o1xagFz.gif)

![График](image-1.png)
![Ошибки](image.png)


CREATE `.env` file similar to `.env.example`

Example `.env` configuration:
```
DATABASE_URL=postgres://postgres:admin@192.168.1.119:5433/test
APP_PORT=8000
RATE_LIMIT_CAPACITY=15
RATE_LIMIT_REFILL_RATE=5
```

Run tests with:
```
locust -f locustfile.py
```
