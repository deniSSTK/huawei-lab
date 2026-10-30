# Testing 2 — Negative Test Protocol

Виконано 27.10.2026, 21:50:43–21:51:55, Europe/Kiev.
Baseline: tag `v1.0.0`, commit `e601531989e28694bbddf5b43fceb88154d55daa`.
Відстежувані файли Compose, gateway та client не змінювалися.

| Scenario | Фактична перевірка | Status | Evidence |
| --- | --- | --- | --- |
| NT-01 / R-02 / TC-02 | Сервіс Corporate TCP/8080 відповів зі шлюзу; Guest отримав curl timeout 5007 мс, exit 28; guest_to_corporate 0 → 5. | PASS | NT-01.log |
| NT-02 / R-07 / TC-07 | Той самий сервіс відповів зі шлюзу; IoT отримав wget download timed out, exit 1; iot_to_corporate 0 → 5. | PASS | NT-02.log |
| NT-03 / R-02, R-07 | Початковий стан, restart Gateway та force-recreate контейнерів: ізоляція й HTTP/HTTPS Guest успішні. Після кожного відновлення drop-лічильники 0 → 3; web 0 → 6 → 19 після restart і 0 → 6 → 18 після recreate. | PASS | NT-03.log |
| NT-04 / R-04, R-08 | Портал, ролі та сесії відсутні у baseline. Дії автентифікації не виконувалися. | BLOCKED | NT-04.log; environment.log |

Фактично виконано 3 сценарії: PASS 3, FAIL 0. Ще 1 сценарій BLOCKED.
Очікування та обсяг збережено до виконання в `expected.json`.
Повний послідовний вивід команд, дати й exit codes — `execution.log`.
Версії, Compose, baseline та git diff — `environment.log`; SHA-256 — `sha256.json`.
Фрагменти термінального виводу включено в додатки А–Г Word-звіту.

Тестова передумова: окремий тимчасовий nginx на TCP/8080 у контейнері
Internal Site. Його конфігурація створювалася лише всередині контейнера,
listener зупинено після NT-01/NT-02; під час force-recreate тимчасові файли
видалені разом зі старим контейнером. Основні правила firewall не змінені.

Обсяг: альтернативний TCP-порт; UDP не перевірявся. NT-03 перевіряє стан після
готовності контейнерів, а не безперервність захисту під час restart/recreate.
NT-03 повторює мережеві запити як регресію після зміни стану стенда;
попередні результати Testing 1 не використовувалися як нові докази.

D-01: Auth Portal не реалізований. Вплив: неможливо перевірити R-04/R-08;
пріоритет High. Відповідальний: Team 1 / Security Managers.
Заняття 17: реалізувати портал, ролі Student/Teacher/Admin і сесії; зафіксувати
нову baseline; повторити NT-04 та регресію NT-01–NT-03. Початковий BLOCKED
зберегти, retest оформити окремо.

Скрипт: `scripts/check-negative.py`; запускається Python 3 з кореня репозиторію
за працюючого Docker. Він перезапускає gateway і пересоздає контейнери стенда.
Наявні докази не перезаписує: перед наступним прогоном перемістіть каталог
`evidence/testing-2` в окремий архів.
