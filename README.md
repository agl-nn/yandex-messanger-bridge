# Yandex Messenger Bridge

Сервис для интеграции вебхуков с Яндекс Мессенджером. Позволяет отправлять уведомления из любых систем, поддерживающих вебхуки, используя Liquid-шаблоны для форматирования сообщений.

## 🚀 Быстрый старт

### 1. Вход в систему
- Откройте веб-интерфейс по адресу `http://your-server:8080`
- Введите email и пароль (для первого входа: admin@localhost / admin)
- При первом входе система потребует сменить пароль

### 2. Создание интеграции

#### Вариант А: Использовать готовый шаблон
1. Перейдите в раздел **Шаблоны**
2. Выберите подходящий шаблон (например, "Jira", "GitLab" и т.д.)
3. Нажмите "Использовать шаблон"
4. Заполните:
    - Название интеграции
    - ID чата в Яндекс Мессенджере
    - Токен бота
5. Нажмите "Создать интеграцию"

#### Вариант Б: Создать кастомную интеграцию
1. На странице **Шаблоны** нажмите "Создать кастомную интеграцию"
2. Заполните:
    - Название интеграции
    - ID чата
    - Токен бота
    - Liquid-шаблон для форматирования сообщений
3. Нажмите "Создать интеграцию"

### 3. Получение ID чата и токена бота

#### ID чата
1. Откройте Яндекс Мессенджер в браузере
2. Перейдите в нужный чат
3. Скопируйте ID из URL: `https://messenger.yandex.ru/chat/0/0/fa15f152-...`
4. Замените `%2F` на `/` (если есть)

#### Токен бота
1. Создайте бота в Яндекс Мессенджере
2. Получите токен у @BotFather
3. Скопируйте токен

### 4. Настройка вебхука в источнике

После создания интеграции вы получите URL вебхука:
http://your-server:8080/webhook/instance/<ID_интеграции>


Настройте вашу систему (Jira, GitLab, Alertmanager и т.д.) отправлять POST-запросы на этот URL.

### 5. Тестирование интеграции
- На странице **Мои интеграции** нажмите кнопку 🚀 для отправки тестового сообщения
- Для просмотра последнего полученного вебхука нажмите кнопку 🔍

## 👑 Администрирование

### Управление пользователями
1. Перейдите в **Администрирование → Пользователи**
2. Доступные действия:
    - ➕ Создание нового пользователя (email, пароль, роль)
    - ✏️ Редактирование пользователя
    - 🔑 Сброс пароля
    - 🗑️ Удаление пользователя

### Управление шаблонами
1. Перейдите в **Администрирование → Управление шаблонами**
2. Шаблоны содержат Liquid-разметку для форматирования сообщений
3. Можно создавать публичные и приватные шаблоны

## 📝 Liquid-шаблоны

### Синтаксис
```liquid
{{ variable }} - вывод переменной
{% if condition %} ... {% endif %} - условие
{% for item in array %} ... {% endfor %} - цикл
```
### Пример для Jira
<details>
<summary>Нажмите, чтобы увидеть код</summary>

```liquid
{% if issue %}
  {%- if webhookEvent == "jira:issue_created" -%}
🆕 **Новая задача**

  {%- elsif issue_event_type_name == "issue_worklog_deleted" -%}
🗑️ **Удален лог времени**

    {%- if changelog and changelog.items %}
      {%- for item in changelog.items %}
        {%- if item.field == "WorklogTimeSpent" and item.fromString %}
⏱️ **Удалено времени:** {{ item.fromString }}
        {%- endif -%}
      {%- endfor -%}
    {%- endif -%}
    
    {%- if issue.fields.timetracking and issue.fields.timetracking.timeSpent %}
⏱️ **Осталось всего:** {{ issue.fields.timetracking.timeSpent }}
    {%- endif %}
    
    {%- if user %}
👤 **Автор:** {{ user.displayName }}
    {%- endif %}

  {%- elsif webhookEvent == "jira:worklog_created" or webhookEvent == "jira:worklog_updated" or issue_event_type_name == "issue_work_logged" or issue_event_type_name == "issue_worklog_updated" -%}
⏱️ **Добавлен/обновлен лог времени**

    {%- if issue.fields.timetracking and issue.fields.timetracking.timeSpent %}
⏱️ **Всего затрачено:** {{ issue.fields.timetracking.timeSpent }}
    {%- endif %}

    {%- if changelog and changelog.items %}
      {%- for item in changelog.items %}
        {%- if item.field == "timespent" and item.from and item.to %}
          {%- assign timeFrom = item.from | plus: 0 -%}
          {%- assign timeTo = item.to | plus: 0 -%}
          {%- assign timeAdded = timeTo | minus: timeFrom -%}
          {%- assign hoursAdded = timeAdded | divided_by: 3600 -%}
          {%- assign minutesAdded = timeAdded | modulo: 3600 | divided_by: 60 -%}
          
⏱️ **Добавлено:** {% if hoursAdded > 0 %}{{ hoursAdded }}ч {% endif %}{% if minutesAdded > 0 %}{{ minutesAdded }}м{% endif %}
        {%- endif -%}
      {%- endfor -%}
    {%- endif -%}
    
    {%- if user %}
👤 **Автор:** {{ user.displayName }}
    {%- endif %}

  {%- elsif webhookEvent == "jira:issue_updated" -%}
    {%- assign hasComment = false -%}
    {%- assign hasStatusChange = false -%}
    {%- assign hasDescriptionChange = false -%}
    {%- assign hasAssigneeChange = false -%}
    {%- assign hasOtherChanges = false -%}
    
    {%- if comment -%}
      {%- assign hasComment = true -%}
    {%- endif -%}
    
    {%- if changelog -%}
      {%- for item in changelog.items -%}
        {%- if item.field == "status" -%}
          {%- assign hasStatusChange = true -%}
        {%- elsif item.field == "description" -%}
          {%- assign hasDescriptionChange = true -%}
        {%- elsif item.field == "assignee" -%}
          {%- assign hasAssigneeChange = true -%}
        {%- elsif item.field != "updated" and item.field != "updateAuthor" -%}
          {%- assign hasOtherChanges = true -%}
        {%- endif -%}
      {%- endfor -%}
    {%- endif -%}
    
    {%- if hasComment and hasStatusChange == false and hasDescriptionChange == false and hasAssigneeChange == false and hasOtherChanges == false -%}
💬 **Новый комментарий**
    {%- elsif hasStatusChange and hasComment == false and hasDescriptionChange == false and hasAssigneeChange == false and hasOtherChanges == false -%}
🔄 **Статус изменен**
    {%- elsif hasAssigneeChange and hasComment == false and hasStatusChange == false and hasDescriptionChange == false and hasOtherChanges == false -%}
👤 **Исполнитель изменен**
    {%- elsif hasDescriptionChange and hasComment == false and hasStatusChange == false and hasAssigneeChange == false and hasOtherChanges == false -%}
📝 **Описание обновлено**
    {%- else -%}
🔄 **Задача обновлена**
    {%- endif -%}
    
    {%- if changelog and changelog.items.size > 0 %}

**✏️ Детали изменений:**
      {%- for item in changelog.items %}
        {%- if item.field == "status" %}
• **Статус**: {{ item.fromString }} → {{ item.toString }}
        {%- elsif item.field == "assignee" %}
          {%- if item.fromString %}
• **Исполнитель**: {{ item.fromString }} → {{ item.toString }}
          {%- else %}
• **Исполнитель**: Назначен {{ item.toString }}
          {%- endif %}
        {%- elsif item.field == "description" %}
• **Описание**: обновлено
        {%- elsif item.field == "priority" %}
• **Приоритет**: {{ item.fromString }} → {{ item.toString }}
        {%- elsif item.field == "summary" %}
• **Заголовок**: {{ item.toString }}
        {%- elsif item.field != "updated" and item.field != "updateAuthor" and item.field != "WorklogId" and item.field != "WorklogTimeSpent" %}
• **{{ item.field }}**: {{ item.toString }}
        {%- endif -%}
      {%- endfor %}
    {%- endif %}
    
    {%- if comment %}

**💬 Комментарий:**
{{ comment.body }}

👤 {{ comment.author.displayName }}
    {%- endif %}
    
  {%- endif %}

**{{ issue.key }}**: {{ issue.fields.summary }}

👤 **Репортер:** {{ issue.fields.reporter.displayName }}{% if issue.fields.assignee %} → **Исполнитель:** {{ issue.fields.assignee.displayName }}{% endif %}

  {%- if issue.fields.status %}
**Статус:** {{ issue.fields.status.name }} {% if issue.fields.status.statusCategory.key == "done" %}✅{% elsif issue.fields.status.statusCategory.key == "new" %}🆕{% elsif issue.fields.status.statusCategory.key == "indeterminate" %}⏳{% endif %}
  {%- endif %}

  {%- if webhookEvent != "jira:worklog_created" and webhookEvent != "jira:worklog_updated" and issue_event_type_name != "issue_work_logged" and issue_event_type_name != "issue_worklog_updated" and issue_event_type_name != "issue_worklog_deleted" and issue.fields.description %}

**📝 Описание:**
{{ issue.fields.description }}
  {%- endif %}

🔗 https://jira.fabrikant.ru/browse/{{ issue.key }}
{% endif %}
```
</details>

### Пример для GitLab (интеграция через MS teams)

<details>
<summary>Нажмите, чтобы увидеть код</summary>

```liquid
{% if attachments and attachments.size > 0 -%}
{% assign card = attachments[0].content -%}
{% if card.type == 'AdaptiveCard' and card.body -%}
📌 **ИНФОРМАЦИЯ О СОБЫТИИ**

{% for item in card.body -%}
{% if item.type == 'TextBlock' and item.weight == 'bolder' -%}
🔹 **{{ item.text }}**

{% elsif item.type == 'TextBlock' and item.weight != 'bolder' -%}
{% if item.isSubtle == true -%}
📍 _{{ item.text }}_

    {% else -%}
📋 {{ item.text }}

    {% endif -%}
{% elsif item.type == 'ColumnSet' -%}
{% for column in item.columns -%}
{% for subitem in column.items -%}
{% if subitem.type == 'TextBlock' -%}
{% if subitem.weight == 'bolder' -%}
👤 **{{ subitem.text }}**

          {% elsif subitem.isSubtle == true -%}
📍 _{{ subitem.text }}_

          {% else -%}
📋 {{ subitem.text }}

          {% endif -%}
        {% endif -%}
      {% endfor -%}
    {% endfor -%}
{% elsif item.type == 'Container' -%}
{% for subitem in item.items -%}
{% if subitem.type == 'TextBlock' -%}
📦 {{ subitem.text }}

      {% endif -%}
    {% endfor -%}
{% endif -%}
{% endfor -%}
---
⏱️ {{ 'now' | date: '%Y-%m-%d %H:%M:%S' }}
{% endif -%}
{% endif -%}
```
</details>

### Пример для Alertmanager

<details>
<summary>Нажмите, чтобы увидеть код</summary>

```liquid
{% for alert in alerts %}{% if alert.status == "firing" and alert.labels.severity == "critical" %}🔥 {{ alert.labels.alertname }} 🔥{% elsif alert.status == "firing" and alert.labels.severity != "critical" %}⚠️ {{ alert.labels.alertname }} ⚠️{% else %}✅ {{ alert.labels.alertname }} ✅{% endif %}

Labels:{% for label in alert.labels %}{% assign key = label[0] %}{% assign value = label[1] %}{% if key != "monitor" and key != "alertname" and key != "instance" and key != "group" and key != "pool" and key != "scrape_uri" and key != "function" and key != "state" and key != "exported_ip" and key != "ip_version" and key != "script" and key != "alertgroup" and key != "slo_trial_version" and key != "slo_service_hostname_vm" and key != "slo_service_fpm_application_metrics" and key != "slo_name" and key != "slo_id" and key != "slo_window" %}
    {% if key == "severity" and value == "critical" %}    🆘  {{ key }}: {{ value }}{% elsif key == "severity" and value == "warning" %}    ⚠️  {{ key }}: {{ value }}{% elsif key == "ip" %}    🔢  {{ key }}: {{ value }}{% elsif key == "hostname" %}    🔡  {{ key }}: {{ value }}{% elsif key == "job" %}    🧐  {{ key }}: {{ value }}{% elsif key == "server_name" %}    💻  {{ key }}: {{ value }}{% elsif key == "environment" %}    🌏  {{ key }}: {{ value }}{% elsif key == "minion" %}    🤓  {{ key }}: {{ value }}{% elsif key == "mountpoint" %}    ⛰️  {{ key }}: {{ value }}{% elsif key == "device" %}    📲  {{ key }}: {{ value }}{% elsif key == "fstype" %}    🗂️  {{ key }}: {{ value }}{% elsif key == "healthcheck" %}    🤧  {{ key }}: {{ value }}{% elsif key == "proxy" %}    🌐  {{ key }}: {{ value }}{% elsif key == "server" %}    🌐  {{ key }}: {{ value }}{% elsif key == "vhost" %}    🐰  {{ key }}: {{ value }}{% elsif key == "queue" %}    🐰  {{ key }}: {{ value }}{% elsif key == "target" %}    🏓  {{ key }}: {{ value }}{% elsif key == "app" %}    🈸  {{ key }}: {{ value }}{% elsif key == "indexType" %}    📇  {{ key }}: {{ value }}{% elsif key == "vm_service" %}    🇻  {{ key }}: {{ value }}{% elsif key == "name" %}    📛  {{ key }}: {{ value }}{% else %}    {{ key }}: {{ value }}{% endif %}{% endif %}{% endfor %}

Annotations:{% for annotation in alert.annotations %}{% assign key = annotation[0] %}{% assign value = annotation[1] %}
    {% if key == "description" %}    📖  {{ key }}: {{ value }}{% elsif key == "WHAT_TO_DO" %}    ❓  {{ key }}: {{ value }}{% elsif key == "supervisor" %}    🕵️  {{ key }}: {{ value }}{% elsif key == "value" %}    📊  {{ key }}: {{ value }}{% else %}    {{ key }}: {{ value }}{% endif %}{% endfor %}

{% if alert.status == "firing" %}Start: {{ alert.startsAt | date: "%Y-%m-%d %H:%M:%S" }}{% else %}Started: {{ alert.startsAt | date: "%Y-%m-%d %H:%M:%S" }}
Ended: {{ alert.endsAt | date: "%Y-%m-%d %H:%M:%S" }}{% endif %}

{% endfor %}
```
</details>

### Генерация секретов
Перед установкой сгенерируйте необходимые секреты:

```bash
# Генерация JWT_SECRET (32 байта)
JWT_SECRET=$(openssl rand -base64 32)
echo "JWT_SECRET: $JWT_SECRET"

# Генерация ENCRYPTION_KEY (32 байта для AES-256)
ENCRYPTION_KEY=$(openssl rand -base64 32)
echo "ENCRYPTION_KEY: $ENCRYPTION_KEY"

# Генерация пароля для базы данных
DB_PASSWORD=$(openssl rand -base64 16)
echo "DB_PASSWORD: $DB_PASSWORD"
```

### ❓ Часто задаваемые вопросы
Q: Не приходят сообщения в чат

Проверьте правильность ID чата и токена бота

Убедитесь, что бот добавлен в чат

Проверьте логи приложения: kubectl logs -f deployment/yandex-bridge -n yandex-bridge

Q: Ошибка "Invalid JSON" при отправке вебхука

Убедитесь, что источник отправляет валидный JSON

Проверьте формат данных в документации источника

Q: Не работает Liquid-шаблон

Проверьте синтаксис на официальном сайте Liquid

Используйте кнопку просмотра последнего вебхука (🔍) чтобы увидеть структуру данных

### Технические детали
Язык: Go 1.23

База данных: от PostgreSQL 15

Шаблонизация: Liquid

Фронтенд: HTMX + TailwindCSS + Alpine.js

Авторизация: JWT + HttpOnly cookies

Контейнеризация: Docker

Оркестрация: Kubernetes (Helm)


