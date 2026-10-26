# 📊 Результати перевірок Sprint 2

| Перевірка | Команда | Очікуваний результат | Фактичний результат | Статус | Файл доказу |
| :--- | :--- | :--- | :--- | :---: | :--- |
| **Guest → Corporate** | `sh scripts/check-isolation.sh` | Блокування трафіку | Заблоковано, лічильник зріс (`packets 3`) | ✅ Passed | `initial-isolation-2026-10-26.txt` |
| **IoT → Corporate** | `sh scripts/check-isolation.sh` | Блокування трафіку | Заблоковано, лічильник зріс (`packets 3`) | ✅ Passed | `initial-isolation-2026-10-26.txt` |
| **Guest Internet** | *Фінальна версія* | Доступ до інтернету | *Очікує фінальної версії* | ⏳ Pending | *Заповнити* |
| **Internal Site** | *Фінальна версія* | Доступ лише дозволеним | Працює для Corporate (`nginx welcome`) | ✅ Passed | `initial-isolation-2026-10-26.txt` |
