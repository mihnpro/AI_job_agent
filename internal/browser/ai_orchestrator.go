package browser

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mihnpro/Ai_agent_2/internal/llm"
	"github.com/mihnpro/Ai_agent_2/internal/prompt"
	"github.com/mihnpro/Ai_agent_2/internal/types"
	playwright "github.com/playwright-community/playwright-go"
)

type AIOrchestrator struct {
	llmClient         *llm.OllamaClient
	page              playwright.Page
	context           playwright.BrowserContext
	debug             bool
	currentTask       string
	actionHistory     []map[string]interface{}
	resume            types.Resume
	lastEvaluatedJobs []types.Job
}

type AIAction struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Reason     string                 `json:"reason"`
	NextStep   string                 `json:"next_step,omitempty"`
}

type JobEvaluationResult struct {
	IsRelevant     bool     `json:"is_relevant"`
	Reason         string   `json:"reason"`
	MatchScore     int      `json:"match_score"`
	Recommendation string   `json:"recommendation"`
	Analysis       string   `json:"analysis"`
	SkillsMatch    []string `json:"skills_match"`
	Requirements   []string `json:"requirements"`
}


func CreateLLMOrchestrator(modelURL, modelName string, page playwright.Page, ctx playwright.BrowserContext, debug bool) *AIOrchestrator {
	client := llm.NewOllamaClient(modelURL, modelName)
	resume := loadResume()

	return &AIOrchestrator{
		llmClient:     client,
		page:          page,
		context:       ctx,
		debug:         debug,
		actionHistory: []map[string]interface{}{},
		resume:        resume,
	}
}

func (a *AIOrchestrator) ExecuteTask(task string, autoApply bool) (string, error) {
	log.Printf("🤖 AI Agent processing task: %s (autoApply: %v)", task, autoApply)
	a.currentTask = task

	plan, err := a.createPlan(task, autoApply)
	if err != nil {
		return "", fmt.Errorf("plan creation failed: %w", err)
	}

	log.Printf("📋 AI создал план из %d шагов", len(plan))

	result, err := a.executePlan(plan)
	if err != nil {
		return "", fmt.Errorf("plan execution failed: %w", err)
	}

	finalReport := a.generateFinalReport()
	
	return finalReport + "\n\n" + result, nil
}

func (a *AIOrchestrator) createPlan(task string, autoApply bool) ([]AIAction, error) {
	promptText := fmt.Sprintf(`Пользователь хочет: "%s"

Проанализируй задачу пользователя и создай пошаговый план для поиска и анализа вакансий на HH.ru.

Контекст:
- Авто-отклик включен: %v
- Есть резюме: %s
- Задача: %s

Создай план в формате JSON массив действий. 

Пример плана для поиска Python разработчика:
[
    {
        "action": "navigate",
        "parameters": {"url": "https://hh.ru"},
        "reason": "Перейти на главную страницу HH.ru"
    },
    {
        "action": "search",
        "parameters": {"query": "Python разработчик Москва"},
        "reason": "Выполнить поиск по запросу пользователя"
    },
    {
        "action": "extract_jobs",
        "parameters": {"count": 10},
        "reason": "Извлечь первые 10 вакансий"
    },
    {
        "action": "evaluate_jobs",
        "parameters": {},
        "reason": "Проанализировать вакансии с помощью AI"
    },
    {
        "action": "apply_to_relevant",
        "parameters": {},
        "reason": "Откликнуться на подходящие вакансии"
    }
]

Только JSON, без дополнительного текста.`, task, autoApply, a.resume.Title, task)

	response, err := a.llmClient.Generate(promptText, prompt.GetSystemPrompt())
	if err != nil {
		return nil, err
	}

	// Извлекаем JSON из ответа
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("invalid plan response: %s", response)
	}

	jsonStr := response[jsonStart : jsonEnd+1]
	var plan []AIAction
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("parse plan JSON: %w", err)
	}

	return plan, nil
}

func (a *AIOrchestrator) executePlan(plan []AIAction) (string, error) {
	var results []string
	
	for i, action := range plan {
		log.Printf("🔧 Шаг %d/%d: %s", i+1, len(plan), action.Reason)

		result, err := a.executeAction(action)
		if err != nil {
			log.Printf("⚠️ Ошибка на шаге %d: %v", i+1, err)
			results = append(results, fmt.Sprintf("❌ Шаг %d: %s - Ошибка: %v", 
				i+1, action.Reason, err))
			continue
		}

		// Записываем в историю
		a.actionHistory = append(a.actionHistory, map[string]interface{}{
			"step":      i + 1,
			"action":    action.Action,
			"reason":    action.Reason,
			"result":    result,
			"timestamp": time.Now().Format(time.RFC3339),
		})

		results = append(results, fmt.Sprintf("✅ Шаг %d: %s - %s", 
			i+1, action.Reason, result))

		time.Sleep(1 * time.Second)
	}

	return strings.Join(results, "\n"), nil
}

func (a *AIOrchestrator) executeAction(action AIAction) (string, error) {
	switch action.Action {
	case "navigate":
		url, ok := action.Parameters["url"].(string)
		if !ok {
			return "", fmt.Errorf("URL not provided for navigation")
		}
		return a.navigate(url)

	case "search":
		query, ok := action.Parameters["query"].(string)
		if !ok {
			query = extractQueryFromTask(a.currentTask)
		}
		return a.searchJobs(query)

	case "extract_jobs":
		count := 10 // default
		if val, ok := action.Parameters["count"]; ok && val != nil {
			count = int(val.(float64))
		}
		jobs, err := a.extractJobsFromPage(count)
		if err != nil {
			return "", err
		}
		a.lastEvaluatedJobs = jobs
		return fmt.Sprintf("Извлечено %d вакансий", len(jobs)), nil

	case "evaluate_jobs":
		return a.evaluateAllJobsOnPage()

	case "apply_to_relevant":
		return a.applyToRelevantJobs()

	case "analyze_page":
		return a.analyzePage()

	case "check_login":
		return a.checkLogin()

	case "solve_captcha":
		return a.handleCaptcha()

	default:
		return a.handleGenericAction(action.Action, action.Parameters)
	}
}


func (a *AIOrchestrator) applyToRelevantJobs() (string, error) {
	log.Println("📨 Откликаюсь на релевантные вакансии...")
	
	if len(a.lastEvaluatedJobs) == 0 {
		log.Println("ℹ️ Нет оцененных вакансий, извлекаю 5 вакансий...")
		jobs, err := a.extractJobsFromPage(5)
		if err != nil {
			return "", fmt.Errorf("не удалось извлечь вакансии: %w", err)
		}
		a.lastEvaluatedJobs = jobs
	}
	
	appliedCount := 0
	skippedCount := 0
	errorCount := 0
	
	var results []string
	
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🧠 НАЧИНАЮ АНАЛИЗ ВАКАНСИЙ И АВТООТКЛИКИ")
	fmt.Println(strings.Repeat("=", 80))
	
	for i, job := range a.lastEvaluatedJobs {
		log.Printf("🤔 Анализирую вакансию %d/%d: %s", 
			i+1, len(a.lastEvaluatedJobs), job.Title)
		
		if !strings.Contains(job.URL, "/vacancy/") || job.URL == "" {
			log.Printf("⚠️ Пропускаем невалидную ссылку: %s", job.URL)
			skippedCount++
			continue
		}
		
		log.Println("🧠 Запрашиваю анализ у AI модели...")
		evaluation, err := a.evaluateJobForTaskWithAnalysis(job, a.currentTask)
		if err != nil {
			log.Printf("❌ Ошибка анализа: %v", err)
			results = append(results, fmt.Sprintf("❌ %s: ошибка анализа", job.Title))
			errorCount++
			continue
		}
		
		a.printJobAnalysis(job, evaluation)

		shouldApply := evaluation.IsRelevant && evaluation.MatchScore >= 7
		
		if shouldApply {
			log.Printf("✅ Решение: откликнуться (оценка: %d/10)", evaluation.MatchScore)
			

			applyResult, err := a.performJobApplication(job)
			if err != nil {
				log.Printf("❌ Ошибка отклика: %v", err)
				results = append(results, fmt.Sprintf("❌ %s: ошибка отклика", job.Title))
				errorCount++
			} else {
				appliedCount++
				results = append(results, fmt.Sprintf("✅ %s: отклик отправлен!", job.Title))
				log.Printf("🎉 Отклик отправлен! Результат: %s", applyResult)
			}
		} else {
			log.Printf("⏭️ Решение: пропустить (оценка: %d/10)", evaluation.MatchScore)
			skippedCount++
			results = append(results, fmt.Sprintf("⏭️ %s: оценка %d/10", 
				job.Title, evaluation.MatchScore))
		}
		
		fmt.Println() 
		time.Sleep(3 * time.Second) 
	}
	
	// Финальный отчет
	finalResult := fmt.Sprintf(`
📊 ИТОГ АНАЛИЗА ВАКАНСИЙ
%s
Проанализировано: %d вакансий
Отправлено откликов: %d
Пропущено: %d
Ошибок: %d
%s
%s`,
		strings.Repeat("=", 50),
		len(a.lastEvaluatedJobs), 
		appliedCount, 
		skippedCount, 
		errorCount,
		strings.Repeat("=", 50),
		strings.Join(results, "\n"))
	
	fmt.Println(finalResult)
	return finalResult, nil
}


func (a *AIOrchestrator) performJobApplication(job types.Job) (string, error) {
	log.Printf("📝 Начинаю отклик на вакансию: %s", job.Title)
	

	page, err := a.context.NewPage()
	if err != nil {
		return "", fmt.Errorf("не удалось создать страницу: %w", err)
	}
	defer page.Close()


	log.Printf("🌐 Перехожу на: %s", job.URL)
	_, err = page.Goto(job.URL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateLoad,
	})
	if err != nil {
		return "", fmt.Errorf("не удалось перейти на вакансию: %w", err)
	}

	time.Sleep(2 * time.Second)


	title, _ := page.Locator("h1").First().TextContent()
	companyLocator := page.Locator("[data-qa='vacancy-company-name'], .vacancy-company-name")
	company, _ := companyLocator.First().TextContent()

	coverLetter, err := a.generateCoverLetter(title, company, job)
	if err != nil {
		log.Printf("⚠️ Не удалось сгенерировать письмо: %v", err)
		coverLetter = "Здравствуйте! Заинтересован(а) в этой вакансии. Готов(а) обсудить детали. С уважением."
	}

	applyBtnSelectors := []string{
		"button:has-text('Откликнуться')",
		"[data-qa='vacancy-response-link-top']",
		"[data-qa='vacancy-serp__vacancy_response']",
		".bloko-button:has-text('Откликнуться')",
	}

	var applyBtn playwright.Locator
	for _, selector := range applyBtnSelectors {
		locator := page.Locator(selector)
		if count, _ := locator.Count(); count > 0 {
			applyBtn = locator.First()
			break
		}
	}

	if applyBtn == nil {
		return "", fmt.Errorf("кнопка отклика не найдена")
	}

	log.Println("🖱️ Нажимаю кнопку 'Откликнуться'...")
	if err := applyBtn.Click(); err != nil {
		return "", fmt.Errorf("не удалось нажать кнопку отклика: %w", err)
	}

	time.Sleep(2 * time.Second)

	messageFieldSelectors := []string{
		"textarea[name='message']",
		"textarea[data-qa='vacancy-response-popup-form-letter-input']",
		"textarea",
	}

	for _, selector := range messageFieldSelectors {
		field := page.Locator(selector)
		if count, _ := field.Count(); count > 0 {
			if err := field.First().Fill(coverLetter); err == nil {
				log.Println("✅ Сопроводительное письмо заполнено")
				break
			}
		}
	}


	submitBtnSelectors := []string{
		"button:has-text('Отправить отклик')",
		"button:has-text('Отправить')",
		"[data-qa='vacancy-response-submit-popup']",
	}

	for _, selector := range submitBtnSelectors {
		submitBtn := page.Locator(selector)
		if count, _ := submitBtn.Count(); count > 0 {
			if err := submitBtn.First().Click(); err == nil {
				log.Println("📤 Отправляю отклик...")
				time.Sleep(3 * time.Second)
				

				successSelectors := []string{
					":has-text('отправлен')",
					":has-text('успешно')",
					"[data-qa='vacancy-response-popup-success']",
				}
				
				for _, successSel := range successSelectors {
					if count, _ := page.Locator(successSel).Count(); count > 0 {
						return fmt.Sprintf("Отклик успешно отправлен на '%s' в '%s'", title, company), nil
					}
				}
				
				return "Отклик отправлен (статус не подтвержден)", nil
			}
		}
	}

	return "Отклик не отправлен (не удалось найти кнопку отправки)", nil
}

func (a *AIOrchestrator) printJobAnalysis(job types.Job, evaluation *JobEvaluationResult) {

	var scoreColor string
	var scoreEmoji string
	
	switch {
	case evaluation.MatchScore >= 9:
		scoreColor = "🟢" 
		scoreEmoji = "🔥"
	case evaluation.MatchScore >= 7:
		scoreColor = "🟡" 
		scoreEmoji = "👍"
	case evaluation.MatchScore >= 5:
		scoreColor = "🟠" 
		scoreEmoji = "🤔"
	default:
		scoreColor = "🔴" 
		scoreEmoji = "❌"
	}

	output := fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                              АНАЛИЗ ВАКАНСИИ                                ║
╠══════════════════════════════════════════════════════════════════════════════╣
║  📝 Должность: %s
║  🏢 Компания:  %s
║  %s Оценка:    %d/10 %s
║  📊 Релевантность: %v
╠══════════════════════════════════════════════════════════════════════════════╣
║  🧠 АНАЛИЗ МОДЕЛИ:
║  %s
╠══════════════════════════════════════════════════════════════════════════════╣
║  ✅ Совпадающие навыки: %s
║  📋 Основные требования: %s
║  🤔 Рекомендация: %s
║  🔗 Ссылка: %s
╚══════════════════════════════════════════════════════════════════════════════╝`,
		truncate(job.Title, 60),
		truncate(job.Company, 50),
		scoreColor,
		evaluation.MatchScore,
		scoreEmoji,
		evaluation.IsRelevant,
		evaluation.Analysis,
		strings.Join(evaluation.SkillsMatch, ", "),
		strings.Join(evaluation.Requirements, ", "),
		getRecommendationText(evaluation.Recommendation),
		truncate(job.URL, 70))

	fmt.Println(output)
}



func (a *AIOrchestrator) evaluateJobForTaskWithAnalysis(job types.Job, userTask string) (*JobEvaluationResult, error) {

	prompt := fmt.Sprintf(`Проанализируй вакансию и дай развернутый анализ.

ЗАДАЧА ПОЛЬЗОВАТЕЛЯ: "%s"

ИНФОРМАЦИЯ О ВАКАНСИИ:
- Должность: %s
- Компания: %s
- Описание: %s

Информация из резюме кандидата:
- Позиция: %s
- Навыки: %s
- Опыт: %s

ПРОАНАЛИЗИРУЙ ПО КРИТЕРИЯМ:
1. Соответствие должности запросу пользователя
2. Соответствие требуемых навыков навыкам кандидата
3. Уровень опыта (junior/middle/senior)
4. Зарплатные ожидания (если указана зарплата)
5. Локация и формат работы (офис/удаленка)
6. Плюсы и минусы вакансии

ВЕРНИ ОТВЕТ В ФОРМАТЕ JSON:
{
    "is_relevant": true/false,
    "reason": "краткое обоснование релевантности",
    "match_score": число от 1 до 10,
    "recommendation": "apply/skip/review",
    "analysis": "развернутый анализ на 3-4 предложения",
    "skills_match": ["Python", "Django", "SQL"],
    "requirements": ["Опыт работы 2+ года", "Знание Python"]
}

Важно: Будь конкретен и объективен в анализе.`,
		userTask,
		job.Title,
		job.Company,
		truncate(job.Snippet, 500),
		a.resume.Title,
		strings.Join(a.resume.Skills, ", "),
		strings.Join(a.resume.Experience, "; "))

	response, err := a.llmClient.Generate(prompt, "Ты эксперт по анализу IT-вакансий. Давай детальный анализ с конкретными аргументами.")
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации анализа: %w", err)
	}

	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("не удалось найти JSON в ответе: %s", response)
	}

	jsonStr := response[jsonStart : jsonEnd+1]
	var result JobEvaluationResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %w\nОтвет: %s", err, response)
	}

	return &result, nil
}

func (a *AIOrchestrator) evaluateAllJobsOnPage() (string, error) {
	log.Println("🧠 Начинаю анализ всех вакансий на странице...")

	jobs, err := a.extractJobsFromPage(10)
	if err != nil {
		return "", fmt.Errorf("не удалось извлечь вакансии: %w", err)
	}

	log.Printf("📊 Найдено %d вакансий для анализа", len(jobs))

	var evaluations []string
	relevantCount := 0

	for i, job := range jobs {
		log.Printf("🔍 Анализ вакансии %d/%d: %s", i+1, len(jobs), job.Title)

		evaluation, err := a.evaluateJobForTaskWithAnalysis(job, a.currentTask)
		if err != nil {
			log.Printf("⚠️ Ошибка оценки: %v", err)
			evaluations = append(evaluations, fmt.Sprintf("❌ %s: ошибка анализа", job.Title))
			continue
		}

		status := "⏭️"
		if evaluation.IsRelevant {
			status = "✅"
			relevantCount++
		}

		evaluations = append(evaluations, fmt.Sprintf("%s %s | %s | Оценка: %d/10 | %s",
			status, job.Title, job.Company, evaluation.MatchScore, evaluation.Reason))

		a.lastEvaluatedJobs = jobs

		time.Sleep(1 * time.Second)
	}

	result := fmt.Sprintf("\n📊 РЕЗУЛЬТАТЫ АНАЛИЗА\n%s\nПроанализировано: %d вакансий\nРелевантных: %d\n\n%s",
		strings.Repeat("=", 50),
		len(jobs), 
		relevantCount,
		strings.Join(evaluations, "\n"))

	return result, nil
}


func getRecommendationText(rec string) string {
	switch strings.ToLower(rec) {
	case "apply":
		return "✅ ОТКЛИКНУТЬСЯ"
	case "skip":
		return "⏭️ ПРОПУСТИТЬ"
	case "review":
		return "🤔 ТРЕБУЕТ ДОП. АНАЛИЗА"
	default:
		return rec
	}
}

func (a *AIOrchestrator) generateCoverLetter(title, company string, job types.Job) (string, error) {
	prompt := fmt.Sprintf(`Создай сопроводительное письмо для отклика на вакансию.

ВАКАНСИЯ:
- Должность: %s
- Компания: %s

МОЕ РЕЗЮМЕ:
- Имя: %s
- Целевая позиция: %s
- Опыт: %s
- Ключевые навыки: %s

ТРЕБОВАНИЯ К ПИСЬМУ:
1. Профессиональное и вежливое
2. Краткое (3-4 предложения)
3. Подчеркивает релевантный опыт
4. Выражает интерес к позиции
5. Упоминает ключевые навыки из вакансии

Напиши только текст письма, без приветствий вроде "Письмо:"`,
		title, company,
		a.resume.Name,
		a.resume.Title,
		strings.Join(a.resume.Experience, "; "),
		strings.Join(a.resume.Skills, ", "))

	letter, err := a.llmClient.Generate(prompt, "Ты пишешь сопроводительные письма для IT-специалистов. Будь кратким и конкретным.")
	if err != nil {
		return "", err
	}

	letter = strings.TrimSpace(letter)
	letter = strings.TrimPrefix(letter, "Письмо:")
	letter = strings.TrimPrefix(letter, "Сопроводительное письмо:")
	letter = strings.TrimSpace(letter)

	if letter == "" {
		letter = "Здравствуйте! Заинтересован(а) в вакансии. Имею релевантный опыт и навыки. Готов(а) обсудить детали."
	}

	return letter, nil
}

func (a *AIOrchestrator) generateFinalReport() string {
	if len(a.actionHistory) == 0 {
		return "Нет данных для отчета"
	}

	var appliedJobs []string
	var analyzedCount int

	for _, action := range a.actionHistory {
		if action["action"] == "apply_to_relevant" || action["action"] == "apply_job" {
			if result, ok := action["result"].(string); ok {
				if strings.Contains(result, "отправлен") {
					appliedJobs = append(appliedJobs, result)
				}
			}
		}
	}

	analyzedCount = len(a.lastEvaluatedJobs)

	report := fmt.Sprintf(`
📋 ФИНАЛЬНЫЙ ОТЧЕТ ВЫПОЛНЕНИЯ ЗАДАЧИ
%s
Задача: %s
Выполнено шагов: %d
Проанализировано вакансий: %d
Отправлено откликов: %d
%s`,
		strings.Repeat("=", 60),
		a.currentTask,
		len(a.actionHistory),
		analyzedCount,
		len(appliedJobs),
		strings.Repeat("=", 60))

	if len(appliedJobs) > 0 {
		report += "\n\n✅ ОТПРАВЛЕННЫЕ ОТКЛИКИ:\n" + strings.Join(appliedJobs, "\n")
	}

	return report
}


func (a *AIOrchestrator) navigate(url string) (string, error) {
	log.Printf("🌐 Навигация на %s", url)
	_, err := a.page.Goto(url, playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return "", fmt.Errorf("навигация не удалась: %w", err)
	}

	time.Sleep(2 * time.Second)
	
	if a.debug {
		a.takeScreenshot("navigate_" + sanitizeFilename(url))
	}

	return "Успешно перешли на страницу", nil
}

func (a *AIOrchestrator) searchJobs(query string) (string, error) {
	log.Printf("🔍 Поиск вакансий: %s", query)
	searchURL := fmt.Sprintf("https://hh.ru/search/vacancy?text=%s&area=113", url.QueryEscape(query))

	log.Printf("🌐 Перехожу на URL поиска: %s", searchURL)
	_, err := a.page.Goto(searchURL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(30000),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return "", fmt.Errorf("ошибка перехода на страницу поиска: %w", err)
	}

	time.Sleep(3 * time.Second)
	
	currentURL := a.page.URL()
	log.Printf("📄 Текущий URL: %s", currentURL)

	if !strings.Contains(currentURL, "text=") {
		log.Println("⚠️ Похоже поиск не сработал, пробую через поле ввода...")
		return a.searchViaInputField(query)
	}

	return fmt.Sprintf("Поиск выполнен: %s", query), nil
}

func (a *AIOrchestrator) analyzePage() (string, error) {
	content, err := a.page.Content()
	if err != nil {
		return "", fmt.Errorf("не удалось получить содержимое страницы: %w", err)
	}

	if len(content) > 4000 {
		content = content[:4000]
	}

	promptText := fmt.Sprintf(`Проанализируй содержимое страницы HH.ru и определи:
1. Это страница с результатами поиска или отдельная вакансия?
2. Сколько вакансий на странице примерно?
3. Какая основная тематика вакансий?

Содержимое страницы: %s`, content)

	analysis, err := a.llmClient.Generate(promptText, "Ты анализируешь веб-страницы HH.ru. Будь кратким и точным.")
	if err != nil {
		return "", err
	}

	return analysis, nil
}

func (a *AIOrchestrator) checkLogin() (string, error) {
	loggedIn, err := isLoggedIn(a.page)
	if err != nil {
		return "", err
	}

	if loggedIn {
		return "✅ Пользователь авторизован", nil
	}

	return "⚠️ Требуется авторизация", nil
}

func (a *AIOrchestrator) handleCaptcha() (string, error) {
	log.Println("⚠️ Обнаружена CAPTCHA")
	return "CAPTCHA обнаружена - требуется ручное вмешательство", nil
}

func (a *AIOrchestrator) handleGenericAction(actionName string, params map[string]interface{}) (string, error) {
	log.Printf("🔄 Обрабатываю общее действие: %s", actionName)
	return fmt.Sprintf("Действие '%s' выполнено", actionName), nil
}


func loadResume() types.Resume {
	return types.Resume{
		Name:  "Иван Иванов",
		Title: "AI Инженер / Python Разработчик",
		Experience: []string{
			"3 года опыта в машинном обучении",
			"Разработка NLP моделей",
			"Оптимизация алгоритмов",
			"Проектирование ML систем",
		},
		Skills: []string{
			"Python", "PyTorch", "TensorFlow", "Scikit-learn",
			"Docker", "Kubernetes", "MLOps", "SQL",
			"FastAPI", "Django", "Git", "Linux",
		},
	}
}

func (a *AIOrchestrator) GetActionHistory() []map[string]interface{} {
	return a.actionHistory
}

func (a *AIOrchestrator) takeScreenshot(name string) {
	screenshotPath := fmt.Sprintf("data/screenshots/%s_%d.png", name, time.Now().Unix())
	if _, err := a.page.Screenshot(playwright.PageScreenshotOptions{
		Path:     playwright.String(screenshotPath),
		FullPage: playwright.Bool(true),
	}); err == nil {
		log.Printf("📸 Скриншот сохранен: %s", screenshotPath)
	}
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)
	return re.ReplaceAllString(name, "_")
}


func (a *AIOrchestrator) searchViaInputField(query string) (string, error) {
	searchInput, err := findSearchInput(a.page)
	if err != nil {
		return "", err
	}

	if err := searchInput.Click(playwright.LocatorClickOptions{ClickCount: playwright.Int(3)}); err != nil {
		log.Printf("⚠️ Не удалось кликнуть: %v", err)
	}

	if err := searchInput.Fill(query); err != nil {
		return "", fmt.Errorf("ошибка ввода: %w", err)
	}

	if err := searchInput.Press("Enter"); err != nil {
		return "", fmt.Errorf("ошибка Enter: %w", err)
	}

	time.Sleep(3 * time.Second)
	return "Поиск выполнен через поле ввода", nil
}

func (a *AIOrchestrator) extractJobsFromPage(count int) ([]types.Job, error) {
	log.Printf("🔍 Извлекаю до %d вакансий со страницы...", count)
	var jobs []types.Job

	page := a.page
	
	time.Sleep(2 * time.Second)
	
	noResultsSelector := "text='По запросу ничего не найдено', text='Ничего не найдено'"
	if count, _ := page.Locator(noResultsSelector).Count(); count > 0 {
		return jobs, fmt.Errorf("вакансий не найдено по данному запросу")
	}

	vacancySelectors := []string{
		"[data-qa='serp-item__title']", // Основной селектор HH.ru
		"a[href*='/vacancy/']:visible", 
	}

	for _, selector := range vacancySelectors {
		vacancyLocator := page.Locator(selector)
		
		err := vacancyLocator.First().WaitFor(playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateAttached,
			Timeout: playwright.Float(10000),
		})
		
		if err != nil {
			log.Printf("⚠️ Селектор %s не найден: %v", selector, err)
			continue
		}

		locatorCount, err := vacancyLocator.Count()
		if err != nil {
			log.Printf("⚠️ Ошибка подсчета элементов: %v", err)
			continue
		}

		if locatorCount > 0 {
			log.Printf("✅ Найдено %d элементов по селектору: %s", locatorCount, selector)
			limit := min(locatorCount, count)

			for i := 0; i < limit; i++ {
				item := vacancyLocator.Nth(i)
				

				visible, err := item.IsVisible()
				if err != nil || !visible {
					continue
				}


				href, err := item.GetAttribute("href")
				if err != nil || href == "" {
					continue
				}


				text, err := item.TextContent()
				if err != nil || text == "" {
					continue
				}


				title := cleanJobTitle(text)
				

				company := ""
				

				fullURL := normalizeVacancyURL(href)
				

				if !isRealVacancyURL(fullURL) {
					continue
				}


				snippet := ""

				job := types.Job{
					Title:   title,
					Company: company,
					URL:     fullURL,
					Snippet: snippet,
				}

				jobs = append(jobs, job)

				if a.debug {
					log.Printf("📝 Вакансия %d: %s | %s", i+1, truncate(job.Title, 40), truncate(job.URL, 40))
				}
				

				time.Sleep(100 * time.Millisecond)
			}
			
			if len(jobs) > 0 {
				log.Printf("✅ Успешно извлечено %d вакансий", len(jobs))
				return jobs, nil
			}
		}
	}


	log.Println("⚠️ Основные селекторы не сработали, использую fallback...")
	return a.extractJobsFallback(count)
}


func (a *AIOrchestrator) extractJobsFallback(count int) ([]types.Job, error) {
	var jobs []types.Job
	
	page := a.page
	

	links := page.Locator("a[href*='vacancy']:visible")
	
	linkCount, err := links.Count()
	if err != nil {
		return jobs, fmt.Errorf("не удалось найти ссылки: %w", err)
	}
	
	log.Printf("ℹ️ Найдено %d ссылок с 'vacancy'", linkCount)
	limit := min(linkCount, count)
	
	for i := 0; i < limit; i++ {
		link := links.Nth(i)
		
		href, err := link.GetAttribute("href")
		if err != nil || href == "" {
			continue
		}
		

		if !isRealVacancyURL(href) {
			continue
		}
		
		text, _ := link.TextContent()
		title := cleanJobTitle(text)
		
		job := types.Job{
			Title:   title,
			Company: "",
			URL:     normalizeVacancyURL(href),
			Snippet: "",
		}
		
		jobs = append(jobs, job)
		
		if a.debug {
			log.Printf("🔗 Вакансия %d: %s", i+1, truncate(job.Title, 30))
		}
	}
	
	if len(jobs) == 0 {
		return jobs, fmt.Errorf("не удалось извлечь ни одной вакансии")
	}
	
	return jobs, nil
}



func cleanJobTitle(title string) string {
	if title == "" {
		return ""
	}
	re := regexp.MustCompile(`^Сейчас (смотрят|смотрит) \d+ (человек|человека)`)
	title = re.ReplaceAllString(title, "")
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.ReplaceAll(title, "\t", " ")
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	return strings.TrimSpace(title)
}


