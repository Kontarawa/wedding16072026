# Свадебное приглашение

## Сайт (localhost)


|                            | URL                                                                                                    |
| -------------------------- | ------------------------------------------------------------------------------------------------------ |
| Приглашение (HTTPS, Caddy) | `https://<SERVER_IP>:8443/wedding/invitation/`                                                         |

На проде замени `localhost` на IP или домен.

## Сохранение ответов анкеты в Google Sheets

Ответы анкеты отправляются в Google Таблицу через **Google Apps Script вебхук**.

1) Открой Google Sheet → **Extensions → Apps Script**.
2) Скопируй файл `scripts/google-apps-script.gs` и вставь в редактор Apps Script.
3) В Apps Script: **Project Settings → Script properties** добавь свойство:

- `WEDDING_WEBHOOK_TOKEN` = `<любой_секретный_токен>`

4) **Deploy → New deployment → Web app**

- Execute as: **Me**
- Who has access: **Anyone** (или “Anyone with link”)

Сохрани URL вида `https://script.google.com/macros/s/.../exec`.

5) Передай URL и токен в контейнер:

- `GOOGLE_SHEETS_WEBHOOK_URL`
- `GOOGLE_SHEETS_WEBHOOK_TOKEN`

## API

База: `/wedding/invitation`.


| Метод  | Путь                        | Описание                                         |
| ------ | --------------------------- | ------------------------------------------------ |
| GET    | `/`                         | Редирект на приглашение                          |
| POST   | `/answer`                   | Публичная анкета → `{ok:true}`                   |


Локальная запись `data/rsvp-submissions.jsonl` больше не используется (источник правды — Google Sheets).

## Запуск (HTTPS)

```bash
docker compose up -d --build
```

Порт: **8443** — Caddy с TLS (браузер может предупредить о сертификате).

