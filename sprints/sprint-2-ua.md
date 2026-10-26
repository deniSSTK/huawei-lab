# Спринт 2: основа Firewall

[English version](sprint-2.md)

## Мета та обсяг

Доповнити топологію спринту 1 відтворюваною ізоляцією Guest → Corporate та IoT → Corporate й вебдоступом Guest через шлюз. Це мережева основа для R-01, R-02, R-07 та D-01 із наданих шаблонів. Фінальний Sprint 2 Record для здачі оформлюється після фіксації версії та скріншотів; рішення про Implementation Freeze приймається на занятті 14.

## Реалізація

- `gateway/Dockerfile` встановлює nftables під час збирання образу.
- `client/Dockerfile` додає curl та CA-сертифікати до Guest для перевірки HTTPS.
- `gateway/nftables.conf` керує `inet lab_filter`, а `gateway/nat.conf` — `ip lab_nat`. Entrypoint атомарно завантажує обидві таблиці, не видаляючи сторонніх таблиць і не накопичуючи дублікати правил.
- Лише шлюз підключений до uplink `172.28.40.0/24` за адресою `172.28.40.254`. Його default route проходить через Docker `172.28.40.1`, обраний через `gw_priority: 1` (Compose 2.33.1+). Entrypoint визначає інтерфейс за IP, не покладаючись на назву `eth0`.
- Docker Compose вмикає пересилання IPv4. Entrypoint завантажує правила до успішного healthcheck; помилка конфігурації зупиняє шлюз. Під час запуску Compose інші сервіси очікують його готовності. Цей порядок не гарантує безперервного блокування під час пересоздання шлюзу з уже запущеними клієнтами; перед продовженням тестів перевірте політику повторно.
- Пакети з Guest (`172.28.10.0/24`) та IoT (`172.28.30.0/24`) до Corporate (`172.28.20.0/24`) блокуються до перевірки established/related. Кожне обмеження має іменований лічильник.
- Guest може надсилати TCP 80/443 та DNS UDP/TCP 53 через uplink. `guest_to_internet` рахує вебтрафік, `guest_dns` — транзитний DNS. Адреса Guest маскується на зовнішньому інтерфейсі, а зворотний трафік дозволяється через established/related. Інший новий трафік, зокрема інтернет IoT, заблоковано. Локальні input/output шлюзу не змінені: клієнти можуть перевірити шлюз через ping, а шлюз — звернутися до Nginx.
- Вбудований DNS Docker може обробляти імена окремо. Перевірка інтернету підтверджує фактичні HTTP/HTTPS-відповіді від `example.com` та зростання веблічильника шлюзу; проксі вимкнено, використовується IPv4, перевірка сертифіката та обмеження часу.

Правила діють усередині контейнера шлюзу, а не на хості. Це стенд із довіреними клієнтами: клієнти мають `NET_ADMIN` для налаштування маршрутів. Модель не захищає від навмисної зміни маршрутів або підміни IP учасником.

## Команди перевірки

```bash
docker compose config --quiet
docker compose up -d --build --wait
docker compose ps
sh scripts/check-isolation.sh
sh scripts/check-internet.sh
docker compose restart gateway
docker compose up -d --wait
sh scripts/check-isolation.sh
sh scripts/check-internet.sh
docker compose up -d --force-recreate --wait
sh scripts/check-isolation.sh
sh scripts/check-internet.sh
```

Скрипт завершується помилкою, якщо сайт або шлях до шлюзу не працює, клієнтський HTTP-запит успішний чи відповідний лічильник Firewall не збільшується. Сам таймаут не вважається доказом ізоляції. Під час перевірки не запускайте інший клієнтський трафік, щоб пов'язати лічильники саме з тестовими запитами.

Для скріншотів термінала можна виконати команди вручну:

```bash
docker compose exec gateway nft list table inet lab_filter
docker compose exec internal-site wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec gateway wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec guest-client ping -c 1 172.28.10.254
docker compose exec iot-client ping -c 1 172.28.30.254
docker compose exec guest-client wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec iot-client wget -T 3 -t 1 -qO- http://172.28.20.10
docker compose exec gateway nft list table inet lab_filter
```

HTTP-запити з обох клієнтів мають завершитися помилкою; запити із сайту та шлюзу — успішно. Порівняйте лічильники Firewall до та після клієнтських запитів.

## Стан перевірки

Інтегровані перевірки пройдено 27 жовтня 2026 року після запуску, перезапуску шлюзу та пересоздання всіх контейнерів: Nginx і шлюзи доступні, запити Guest/IoT до Corporate блокуються зі зростанням лічильників, а Guest отримує HTTP/HTTPS-відповіді через Gateway. Див. єдиний [звіт із результатами, середовищем і логами](../evidence/sprint-2/results.md). Повторний збір: `sh scripts/collect-isolation-evidence.sh`.

[Опис порталу та цільова матриця ролей](../docs/auth-portal-ua.md) готові; програмна реалізація й автоматичні рольові дозволи залишаються наступним етапом. Докази зараз — фактичний вивід команд; скріншоти для здачі потрібно додати. Перевірено ізоляцію після готовності контейнерів, а не безперервність блокування під час пересоздання. Переносимість на інші ПК не перевірялася; записаний запуск виконано в OrbStack на macOS.

## Лабораторна 14 Implementation Freeze

1. Зберегти фінальну версію в commit, виконати `git rev-parse HEAD` і записати повний SHA та дату у Freeze Record.
2. Виконати короткий Smoke Check цього commit: `docker compose up -d --build --wait`, `docker compose ps`, `sh scripts/check-isolation.sh`, `sh scripts/check-internet.sh`. Зберегти вивід. Повний restart/recreate повторювати після змін мережевого коду або невдалого Smoke Check.
3. Заповнити наданий Freeze Record: версія, готовність вимог, Smoke Check, відомі проблеми та передача на Testing. Кожен учасник самостійно заповнює свій Contribution Log #4 фактичним внеском.

R-01 готова в межах перевіреного вебдоступу Guest; R-02 та R-07 — для перевірених блокувань тестового HTTP-сервісу Corporate. Для R-04 готовий опис, але відсутня програмна реалізація; R-08 має GAP щодо динамічного доступу за роллю. Точні формулювання потрібно звірити з командним Requirements.

Після успішного Smoke Check обрати `READY WITH KNOWN ISSUES` для мережевого Testing; при невдачі — `HOLD`. На занятті 15 почати з TC-01 (інтернет Guest), TC-02 (блокування Guest) і TC-07 (блокування IoT). Тести входу, ролей, logout та завершення сесій залишаються BLOCKED до реалізації порталу й інтеграції зі шлюзом.
