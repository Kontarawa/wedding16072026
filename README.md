# Wedding invitation (static + Go)

Сервис отдаёт `index.html` и `/health`. Порт по умолчанию: `8080` (переменная `PORT`).

## Требования на сервере

- Docker Engine с плагином Compose v2 (`docker compose`)
- Открытый входящий порт `8080` (или прокси до контейнера)

Установка Docker (Debian/Ubuntu):

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
```

Перелогиниться, чтобы группа `docker` применилась.

## Развёртывание на сервере

1. Скопировать репозиторий на сервер (git clone, rsync, scp — как удобно).

2. В каталоге проекта:

```bash
chmod +x deploy.sh
./deploy.sh
```

3. Проверка: `http://<ip-сервера>:8080/` и `http://<ip-сервера>:8080/health` (ответ `ok`).

4. Логи: `docker compose logs -f`

5. Остановка: `docker compose down`

## Пример с локальной машины

```bash
rsync -avz --delete ./ user@SERVER:/opt/wedding-invitation/
ssh user@SERVER 'cd /opt/wedding-invitation && ./deploy.sh'
```

Перед первым запуском на сервере один раз: `chmod +x /opt/wedding-invitation/deploy.sh`.

## Прокси

За nginx/Caddy укажите `proxy_pass` на `127.0.0.1:8080`, TLS на стороне прокси.
