# Поиск и модели для агентов Tayga

Сначала установите [Tayga с PostgreSQL](setup.md), включите Mattermost и войдите
администратором нужного домена. Подключение модели и поисковый сервер
настраиваются независимо. Для первого теста модели выключите навыки и
инструменты агента. После простого ответа включайте возможности по одной.

## SearXNG на том же сервере

Этот вариант подходит для Debian/Ubuntu с systemd. Установщик из выпуска
фиксирует ревизию SearXNG, создаёт отдельную системную учётную запись и службу
`tms-search`, слушающую `127.0.0.1:18788`. Служба использует системные
сертификаты, собственное окружение Python и приватный файл настроек.

```sh
sudo apt update
sudo apt install -y python3 curl jq
# Для установки Tayga из DEB
sudo bash /usr/share/tayga-mail/agents/install-searxng.sh
# Для установки из архива Tayga выполните вместо команды выше:
# sudo bash scripts/agents/install-searxng.sh
sudo systemctl is-active tms-search
curl -fsS --get http://127.0.0.1:18788/search \
  --data-urlencode 'q=PostgreSQL documentation' \
  --data 'format=json' | jq -e '.results | length > 0'
```

При повторном выполнении установщик сохраняет существующую установку и
останавливается. Настройки находятся в `/opt/tms-search/settings.yml`.
Изменения применяйте через `sudo systemctl restart tms-search`, после чего
повторяйте проверку JSON. Для другого узла установите SearXNG по его
документации, добавьте `json` в `search.formats` и укажите в Tayga
доступный серверу базовый HTTPS URL. Не открывайте loopback-экземпляр в Интернет.
Нулевое число результатов может означать, что настроенные поисковые системы
недоступны или ограничивают запросы IP-адреса сервера. [Установка SearXNG](https://docs.searxng.org/admin/installation-docker.html) · [Search API](https://docs.searxng.org/dev/search_api.html)

1. Откройте **Чаты → Агенты → Настройки → Веб-поиск**.
2. Выберите **SearXNG**, укажите `http://127.0.0.1:18788` без `/search` и
   сохраните. Ключ для этой установки не нужен.
3. В карточке агента разрешите **Поиск в интернете**. Откройте новый диалог,
   включите **Веб-поиск** и попросите найти свежий источник и дать ссылку.
4. Повторите для остальных доменов, где нужен поиск. Одну службу могут
   использовать несколько доменов, каждым подключением управляет его
   администратор. SearXNG передаёт запрос поисковым системам.

## Добавление модели в Tayga

Откройте **Чаты → Агенты → Настройки → Подключить модель**. Выберите
провайдера, заполните **Название**, **Базовый URL API** и **Идентификатор модели**, введите ключ API, если
он нужен. **Проверить подключение** загружает список доступных моделей.
Выберите запись или укажите точный ID вручную. Сохраните подключение,
затем создайте агента и выберите для него это подключение. Каталог проверяет
адрес и доступ к списку. Генерацию и инструменты проверьте в отдельном диалоге.

| Провайдер в Tayga | Базовый URL | Модель и ключ |
| --- | --- | --- |
| OpenAI (ChatGPT) | `https://api.openai.com/v1` | ID из списка проекта OpenAI и ключ OpenAI API |
| DeepSeek | `https://api.deepseek.com` | Например, `deepseek-flash` и ключ DeepSeek API |
| Ollama | `http://127.0.0.1:11434` | Тег из `ollama list`, например `qwen3:4b`. Для локального штатного API ключ пустой |
| Другой OpenAI-совместимый API | `https://models.example.com/v1` | Имя опубликованного маршрута GPUStack и его API-ключ |

Для встроенных OpenAI и DeepSeek и штатного локального Ollama каталог
доступен сразу. Для GPUStack, другого совместимого API и удалённого
Ollama системный администратор разрешает **точный базовый URL**:

```sh
sudo systemctl edit tayga.service
```

```ini
[Service]
Environment="TAYGA_AGENT_MODEL_CATALOG_URLS=https://models.example.com/v1,http://192.168.1.20:11434"
```

```sh
sudo systemctl daemon-reload
sudo systemctl restart tayga.service
```

Подставьте свои адреса, сохранив действующие записи. Добавляйте только
серверы, которым доверяете передавать токен соответствующего провайдера.
Переменная задаёт разрешения на проверку каталога и не содержит ключей.
Ключи записываются через форму домена.

## OpenAI и DeepSeek

Создайте ключ в нужном проекте и убедитесь, что API-аккаунт имеет доступ
к выбранной модели и активную оплату. Для OpenAI используйте встроенный
тип **OpenAI (ChatGPT)**. Он умеет обращаться к GPT‑6.2 с инструментами
через Responses API и поддерживает TLS-проверку через отдельный TCP-маршрут.

В поле URL оставьте официальный адрес. Если OpenAI доступен из вашей
сети через VPN, подготовьте на сервере Tayga прослушивающий только loopback
TCP-туннель до `api.openai.com:443` с выходом через VPN. Tayga использует
адрес туннеля из `TAYGA_OPENAI_RELAY`. TLS завершается на OpenAI. Это настройка
для встроенного типа OpenAI, она не меняет маршрут DeepSeek, SearXNG или почты.
При готовом туннеле добавьте в тот же systemd override:

```ini
[Service]
Environment="TAYGA_OPENAI_RELAY=127.0.0.1:18790"
```

```sh
openssl s_client -connect 127.0.0.1:18790 -servername api.openai.com \
  -verify_hostname api.openai.com -verify_return_error -brief </dev/null
sudo systemctl daemon-reload
sudo systemctl restart tayga.service
```

В форме повторите **Проверить подключение**. Отдельно отправьте «Ответь
одним словом: готов» в новом диалоге. Если каталог отвечает, а генерация
нет, зафиксируйте провайдера, ID модели, код HTTP, время и был ли включён
инструмент. Ключ, заголовки авторизации и переписку в диагностический
запрос не включайте. [OpenAI Platform](https://platform.openai.com/api-keys) · [DeepSeek Platform](https://platform.deepseek.com/) · [DeepSeek API](https://api-docs.deepseek.com/)

## Ollama на сервере Tayga или в локальной сети

Установите Ollama на машине с ресурсами под выбранную модель. На Linux
используйте архив для своей архитектуры или официальный установщик.
Пример для Linux с systemd:

```sh
curl --proto '=https' -fL https://ollama.com/install.sh -o ollama-install.sh
less ollama-install.sh
sh ./ollama-install.sh
sudo systemctl enable --now ollama.service
ollama pull qwen3:4b
ollama list
curl -fsS http://127.0.0.1:11434/api/tags | jq -e '.models[].name'
```

Размер и скорость зависят от модели, квантования, объёма контекста и
доступной RAM/VRAM. Сначала проверьте простой ответ командой
`ollama run qwen3:4b "Ответь одним словом: готов"`. В той же системе Tayga
подключите тип **Ollama** с `http://127.0.0.1:11434`.

На отдельной машине задайте `OLLAMA_HOST=192.168.1.20:11434` в systemd
override службы Ollama и разрешите доступ к порту только серверу Tayga.
После перезапуска проверяйте `/api/tags` уже с сервера Tayga. Нативный
тип Ollama принимает HTTP на loopback и числовом частном адресе.
Для удалённого DNS-имени используйте HTTPS, при необходимости с отдельным
аутентифицирующим прокси. Имя хоста, модель и тип API должны соответствовать
фактическому сервису. [Установка Ollama на Linux](https://docs.ollama.com/linux) · [Ollama Chat API](https://docs.ollama.com/api/chat)

## GPUStack со своим GPU

1. Подготовьте узел с поддерживаемой ОС, драйвером для вашего GPU, Docker и
   контейнерным рантаймом для ускорителя. Установите сервер и добавьте рабочий
   узел по инструкции для вашей версии GPUStack. Для сервера и рабочих узлов
   берите образы одной версии. Сохраните каталог данных сервера, выберите
   отдельный DNS-адрес для API и настройте HTTPS. Ниже приведён запуск
   сервера GPUStack v2 на отдельном узле со свободными 80/443. Укажите
   опубликованный тег своего выпуска и смонтируйте PEM-цепочку и ключ
   для `models.example.com` из каталога `/etc/gpustack/tls`:

   ```bash
   read -r -p 'Тег выпуска GPUStack v2: ' gpustack_tag
   test -n "$gpustack_tag" && sudo docker run -d --name gpustack \
     --restart unless-stopped -p 80:80 -p 443:443 \
     -v gpustack-data:/var/lib/gpustack \
     -v /etc/gpustack/tls:/certs:ro \
     -e GPUSTACK_SSL_CERTFILE=/certs/fullchain.pem \
     -e GPUSTACK_SSL_KEYFILE=/certs/privkey.pem \
     "gpustack/gpustack:$gpustack_tag"
   ```

   Откройте `https://models.example.com`, завершите первоначальный вход
   по документации своей версии и смените пароль. Добавьте кластер и
   запустите полученную в его интерфейсе команду регистрации рабочего
   узла, сохранив её токен в секрете. Дождитесь, пока worker и устройства
   появятся в интерфейсе. [Установка GPUStack через Docker](https://docs.gpustack.ai/latest/installation/installation/)
2. В GPUStack создайте развёртывание чат-модели и дождитесь готовности рабочего
   экземпляра. Если версия GPUStack использует **Model Routes**, создайте
   или проверьте маршрут к развёртыванию. Выберите подходящий движок и модель,
   способные выполнять вызовы функций, если нужны инструменты Tayga.
   [GPUStack Quickstart](https://docs.gpustack.ai/latest/quickstart/) · [Inference APIs](https://docs.gpustack.ai/latest/integrations/inference-apis/)
3. Создайте API-ключ GPUStack для Tayga. Проверьте `GET /v1/models` с
   Bearer-ключом из сети Tayga. Скопируйте нужный `data[].id`. Для первой
   проверки генерации в GPUStack Playground выберите тот же маршрут.
   На сервере Tayga можно проверить каталог из Bash. Ключ считывается
   без отображения и передаётся curl через stdin.

   ```bash
   read -rs -p 'Ключ GPUStack: ' gpustack_api_key
   printf '\n'
   printf 'header = "Authorization: Bearer %s"\n' "$gpustack_api_key" \
     | curl --config - -fsS https://models.example.com/v1/models \
     | jq -e '.data[].id'
   unset gpustack_api_key
   ```

4. В Tayga выберите **Другой OpenAI-совместимый API**, укажите
   `https://models.example.com/v1`, ключ и точный ID. Для кнопки каталога
   добавьте этот базовый URL в `TAYGA_AGENT_MODEL_CATALOG_URLS`. Tayga
   дописывает `/models` и `/chat/completions`, поэтому конечный путь в
   поле базового URL не нужен.
5. Проверьте ответ из нового диалога Tayga. После этого проверьте запрос с
   поиском или с чтением тестового сообщения, если выбранная модель и её
   сервер поддерживают соответствующие function tools.

## Контроль после включения

- Откройте агента под обычным пользователем домена. Убедитесь, что он видит
  только свои диалоги. Повторите проверку под соседним доменом с тестовой
  учётной записью.
- В вебе и Mattermost продолжите одну тестовую ветку. Отдельно проверьте
  веб-поиск со ссылкой на источник, чтение своей тестовой почты или канала и
  черновик с ручным подтверждением, если эти инструменты включены.
- Значение `401`/ `403` от модели проверьте по ключу, доступу проекта и
  адресу. Для `400` проверьте точный ID и ограничения модели на инструменты.
  Для `429` проверьте квоту и лимиты. При тайм-ауте сверьте TLS, задержку
  локальной генерации и нагрузку на модель.
