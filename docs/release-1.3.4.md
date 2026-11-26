# Tayga Mail Server 1.3.4

## Русский

- **Совместимость с PowerPoint.** Новые презентации содержат тему оформления. Ранее сохранённые презентации агентов получают её при повторном скачивании, пока доступен временный файл. Текст и порядок слайдов сохраняются.
- **Генерация изображений.** Для каждого агента можно выбрать отдельное подключение и модель OpenAI Images API. Агент генерирует PNG для скачивания или вставляет их прямо в слайды PowerPoint. Изображения хранятся внутри презентации.

Миграция базы данных не требуется. В выпуск входит расширение Thunderbird 2.5.3.

## English

- **PowerPoint compatibility.** New presentations include a theme. Downloading an existing agent presentation adds the theme while the temporary file is still available, preserving the slide text and order.
- **Image generation.** Each agent can use a separate OpenAI Images API connection and model. It can generate downloadable PNG images or embed them directly in PowerPoint slides. Images are stored inside the presentation.

This release requires no database schema changes and includes Thunderbird 2.5.3.
