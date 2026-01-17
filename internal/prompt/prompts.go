package prompt


func GetSystemPrompt() string {
    return `Ты — AI ассистент для поиска работы на HH.ru. 

ВАЖНЫЕ ИНСТРУКЦИИ:
1. При поиске всегда используй параметр ?text= в URL
2. Для извлечения вакансий используй селектор [data-qa='vacancy-serp__vacancy']
3. Проверяй что ссылки на вакансии содержат /vacancy/
4. Для анализа вакансий учитывай навыки из резюме

Пример правильного плана для "найди 5 вакансий python разработчика":
[
    {
        "action": "navigate",
        "parameters": {"url": "https://hh.ru/search/vacancy?text=python+разработчик&area=113"},
        "reason": "Прямой переход на страницу поиска Python вакансий"
    },
    {
        "action": "extract_jobs",
        "parameters": {"count": 5},
        "reason": "Извлечь 5 первых вакансий"
    },
    {
        "action": "apply_to_relevant",
        "parameters": {},
        "reason": "Откликнуться на подходящие Python вакансии"
    }
]`
}


func GetJobSearchPrompt(task string) string {
    return `Пользователь хочет найти вакансии: "` + task + `"

Проанализируй задачу и определи:
1. Ключевые слова для поиска
2. Критерии релевантности
3. Сколько вакансий нужно найти
4. Нужно ли отправлять отклики

Действуй по плану:
1. Перейди на hh.ru
2. Проверь вход в аккаунт (если требуется)
3. Выполни поиск по ключевым словам
4. Проанализируй результаты
5. Отбери наиболее подходящие вакансии
6. Если разрешено — подготовь отклики`
}


func GetAnalyzeJobPrompt(title, company, description string) string {
    return `Проанализируй вакансию и определи её релевантность.

Информация о вакансии:
- Название: ` + title + `
- Компания: ` + company + `
- Описание: ` + truncate(description, 1000) + `

Критерии релевантности:
1. Соответствие навыкам из резюме
2. Уровень опыта
3. Технологический стек
4. Локация (если указана)

Верни оценку от 1 до 10 и краткое обоснование.`
}


func GetCoverLetterPrompt(position, company, requirements, name, experience, skills string) string {
    return `Создай персонализированное сопроводительное письмо для вакансии.

Информация о вакансии:
- Должность: ` + position + `
- Компания: ` + company + `
- Требования: ` + truncate(requirements, 500) + `

Информация из резюме:
- Имя: ` + name + `
- Опыт: ` + truncate(experience, 200) + `
- Навыки: ` + truncate(skills, 200) + `

Создай письмо, которое:
1. Показывает интерес к конкретной позиции
2. Подчеркивает релевантный опыт
3. Кратко и по делу
4. Профессионально и вежливо`
}


func GetActionPrompt(action string, params ...string) string {
    switch action {
    case "navigate":
        return "Перейди на указанный URL: " + params[0]
    case "search":
        return "Выполни поиск по запросу: " + params[0]
    case "extract":
        return "Извлеки информацию о вакансиях со страницы"
    case "apply":
        return "Подготовь отклик на вакансию: " + params[0]
    case "evaluate":
        return "Оцени релевантность вакансии по критериям: " + params[0]
    default:
        return "Выполни действие: " + action
    }
}


func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}