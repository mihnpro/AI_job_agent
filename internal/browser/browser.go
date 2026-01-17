package browser

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mihnpro/Ai_agent_2/internal/types"
	playwright "github.com/playwright-community/playwright-go"
)


type Config = types.Config
type Job = types.Job

type LLMOrchestratorInterface interface {
	ExecuteTask(task string, autoApply bool) (string, error)
	GetActionHistory() []map[string]interface{}
}




func getRandomUserAgent() string {
	agents := []string{
		// Windows Chrome
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		// Mac Chrome
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
		// Firefox
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
		// Safari
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	}
	return agents[time.Now().UnixNano()%int64(len(agents))]
}


func addStealthScripts(ctx playwright.BrowserContext) error {
	stealthScript := playwright.Script{
		Content: playwright.String(`
    // Удаляем webdriver property
    const originalDescriptor = Object.getOwnPropertyDescriptor(navigator, 'webdriver');
    if (originalDescriptor) {
        delete navigator.webdriver;
        Object.defineProperty(navigator, 'webdriver', {
            get: () => undefined
        });
    }
    
    // Скрываем automation в window
    if (window.chrome) {
        window.chrome = {
            runtime: {},
            loadTimes: function() {},
            csi: function() {},
            app: {}
        };
    }
    
    // Изменяем languages
    Object.defineProperty(navigator, 'languages', {
        get: () => ['ru-RU', 'ru', 'en-US', 'en', 'en-GB']
    });
    
    // Изменяем plugins
    Object.defineProperty(navigator, 'plugins', {
        get: () => {
            return [
                {name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer'},
                {name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai'},
                {name: 'Native Client', filename: 'internal-nacl-plugin'}
            ];
        }
    });
    
    // Изменяем platform
    const originalPlatform = navigator.platform;
    Object.defineProperty(navigator, 'platform', {
        get: () => originalPlatform,
        configurable: true
    });
    
    // Добавляем noise в performance timing
    if (window.performance && window.performance.now) {
        const originalNow = performance.now.bind(performance);
        performance.now = function() {
            return originalNow() + Math.random() * 5;
        };
    }
    
    // Модифицируем permissions
    const originalQuery = navigator.permissions.query;
    navigator.permissions.query = function(parameters) {
        if (parameters.name === 'notifications') {
            return Promise.resolve({
                state: Notification.permission
            });
        }
        return originalQuery(parameters);
    };
    
    // Ложим cookie для обхода проверок
    if (!document.cookie.includes('visited=true')) {
        document.cookie = 'visited=true; max-age=86400; path=/';
    }
    
    console.log('[Stealth] Браузер замаскирован');
    `),
	}

	err := ctx.AddInitScript(stealthScript)

	if err != nil {
		log.Printf("⚠️ Не удалось добавить stealth скрипт: %v", err)
		return err
	}

	log.Println("✅ Stealth скрипты добавлены")
	return nil
}


func humanLikeDelay() {
	delays := []time.Duration{
		800 * time.Millisecond,
		1200 * time.Millisecond,
		1500 * time.Millisecond,
		1800 * time.Millisecond,
		2100 * time.Millisecond,
		2500 * time.Millisecond,
	}
	delay := delays[rand.Intn(len(delays))]
	time.Sleep(delay)
}

func createStealthContext(pw *playwright.Playwright, profileDir string, headless bool, debug bool) (playwright.BrowserContext, error) {

	args := []string{
		"--disable-blink-features=AutomationControlled",
		"--disable-features=IsolateOrigins,site-per-process",
		"--disable-site-isolation-trials",
		"--disable-web-security",
		"--disable-features=BlockInsecurePrivateNetworkRequests",
		"--no-sandbox",
		"--disable-setuid-sandbox",
		"--disable-dev-shm-usage",
		"--disable-accelerated-2d-canvas",
		"--disable-gpu",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-features=AudioServiceOutOfProcess",
		"--disable-features=TranslateUI",
		"--lang=ru-RU,ru",
		"--user-agent=" + getRandomUserAgent(),
		"--disable-webgl",
		"--disable-3d-apis",
		"--disable-reading-from-canvas",
		"--disable-notifications",
	}


	viewports := []playwright.Size{
		{Width: 1920, Height: 1080},
		{Width: 1366, Height: 768},
		{Width: 1536, Height: 864},
		{Width: 1440, Height: 900},
		{Width: 1600, Height: 900},
	}

	viewport := viewports[time.Now().UnixNano()%int64(len(viewports))]


	timezone := "Europe/Moscow"
	if rand.Intn(2) == 0 {
		timezone = "UTC"
	}


	var ctx playwright.BrowserContext
	var err error

	if profileDir != "" {
		absPath, err := filepath.Abs(profileDir)
		if err != nil {
			return nil, fmt.Errorf("could not determine absolute path for profile dir: %w", err)
		}


		if err := os.MkdirAll(absPath, 0755); err != nil {
			return nil, fmt.Errorf("could not create profile dir %s: %w", absPath, err)
		}

		log.Printf("📁 Using persistent profile dir: %s", absPath)


		ctx, err = pw.Chromium.LaunchPersistentContext(
			absPath,
			playwright.BrowserTypeLaunchPersistentContextOptions{
				Headless:          playwright.Bool(headless),
				Args:              args,
				Viewport:          &viewport,
				UserAgent:         playwright.String(getRandomUserAgent()),
				TimezoneId:        playwright.String(timezone), 
				AcceptDownloads:   playwright.Bool(true),
				BypassCSP:         playwright.Bool(true),
				IgnoreHttpsErrors: playwright.Bool(true), 
				JavaScriptEnabled: playwright.Bool(true),
				Locale:            playwright.String("ru-RU"),
				ColorScheme:       playwright.ColorSchemeLight,
				Permissions:       []string{"geolocation"},
			},
		)

		if err != nil {
			if debug {
				log.Printf("⚠️ Failed to create persistent context: %v", err)

				return createRegularContext(pw, headless, debug)
			}
			return nil, fmt.Errorf("could not launch persistent context: %w", err)
		}
	} else {

		ctx, err = createRegularContext(pw, headless, debug)
		if err != nil {
			return nil, err
		}
	}


	if err := addStealthScripts(ctx); err != nil && debug {
		log.Printf("⚠️ Stealth scripts warning: %v", err)
	}


	extraHeaders := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language":           "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
		"Accept-Encoding":           "gzip, deflate, br",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Cache-Control":             "max-age=0",
	}

	ctx.SetExtraHTTPHeaders(extraHeaders)

	log.Printf("✅ Stealth контекст создан: %dx%d, %s",
		viewport.Width, viewport.Height, timezone)

	return ctx, nil
}


func createRegularContext(pw *playwright.Playwright, headless bool, debug bool) (playwright.BrowserContext, error) {
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			"--user-agent=" + getRandomUserAgent(),
		},
	})

	if err != nil {
		return nil, fmt.Errorf("could not launch browser: %w", err)
	}

	viewport := playwright.Size{Width: 1920, Height: 1080}
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Viewport:          &viewport,
		UserAgent:         playwright.String(getRandomUserAgent()),
		AcceptDownloads:   playwright.Bool(true),
		IgnoreHttpsErrors: playwright.Bool(true), 
		Locale:            playwright.String("ru-RU"),
		ColorScheme:       playwright.ColorSchemeLight, 
	})

	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("could not create browser context: %w", err)
	}

	return ctx, nil
}




func RunTask(task string, cfg Config) error {
	log.Printf("Starting task: %s (AI mode: %v, Debug: %v)\n", task, cfg.UseAI, cfg.Debug)


	rand.Seed(time.Now().UnixNano())


	pw, err := playwright.Run()
	if err != nil {
		if strings.Contains(err.Error(), "please install the driver") || strings.Contains(strings.ToLower(err.Error()), "driver") {
			log.Println("Playwright driver not found — attempting automatic install...")
			if ierr := playwright.Install(); ierr != nil {
				return fmt.Errorf("playwright install failed: %w (original: %v)", ierr, err)
			}
			pw, err = playwright.Run()
		}
		if err != nil {
			return fmt.Errorf("could not start playwright: %w", err)
		}
	}
	defer func() {
		if err := pw.Stop(); err != nil {
			log.Printf("playwright stop error: %v", err)
		}
	}()


	ctx, err := createStealthContext(pw, cfg.ProfileDir, cfg.Headless, cfg.Debug)
	if err != nil {
		return fmt.Errorf("could not create stealth context: %w", err)
	}
	defer ctx.Close()


	page, err := ctx.NewPage()
	if err != nil {
		return fmt.Errorf("could not create page: %w", err)
	}


	if cfg.Debug {
		defer func() {
			if _, err := page.Screenshot(playwright.PageScreenshotOptions{
				Path:     playwright.String("data/screenshots/final_state.png"),
				FullPage: playwright.Bool(true),
			}); err == nil {
				log.Printf("📸 Скриншот сохранен: data/screenshots/final_state.png")
			}
		}()
	}


	page.Route("**/*", func(route playwright.Route) {
		request := route.Request()
		url := request.URL()


		blockPatterns := []string{
			"/article/",
			"/special/",
			"/partner/",
			"/promo/",
			"utm_source=hh_lead_gen",
			"hhtmFrom=vacancy_immediate_redirect",
			"utm_campaign=",
			"gazpromneft",
			"doubleclick.net",
			"google-analytics.com",
			"googlesyndication.com",
			"googleads.g.doubleclick.net",
			"mc.yandex.ru",
			"an.yandex.ru",
		}

		for _, pattern := range blockPatterns {
			if strings.Contains(url, pattern) {
				if cfg.Debug {
					log.Printf("🚫 Блокируем рекламу: %s", url)
				}
				route.Abort()
				return
			}
		}


		route.Continue()
	})


	if cfg.ProfileDir != "" {
		loggedIn, err := verifyAndHandleLogin(page, cfg.Headless, cfg.ProfileDir, cfg.AutoApply)
		if err != nil {
			return err
		}

		if !loggedIn && cfg.AutoApply {
			return fmt.Errorf("для автооткликов требуется войти в аккаунт. Используйте --headless=false для интерактивного входа")
		}
	}


	humanLikeDelay()


	if cfg.UseAI {
		log.Println("🔮 Использую AI-агента для выполнения задачи")


		orchestrator := CreateLLMOrchestrator(cfg.ModelURL, cfg.ModelName, page, ctx, cfg.Debug)


		result, err := orchestrator.ExecuteTask(task, cfg.AutoApply)
		if err != nil {
			if cfg.Debug {
				if _, serr := page.Screenshot(playwright.PageScreenshotOptions{
					Path:     playwright.String("data/screenshots/error_state.png"),
					FullPage: playwright.Bool(true),
				}); serr == nil {
					log.Printf("📸 Скриншот ошибки сохранен: data/screenshots/error_state.png")
				}
			}
			return fmt.Errorf("AI agent error: %w", err)
		}

		log.Printf("✅ AI agent completed task: %s", result)


		report := orchestrator.GetActionHistory()
		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))

		return nil
	} else {

		lower := strings.ToLower(task)
		if strings.Contains(lower, "ваканс") || strings.Contains(lower, "hh.ru") || strings.Contains(lower, "hh") {
			query := extractQueryFromTask(task)
			if query == "" {
				query = "AI инженер"
			}
			log.Printf("Extracted search query: %q", query)

			loggedIn := false
			if cfg.ProfileDir != "" {
				if l, lerr := isLoggedIn(page); l && lerr == nil {
					loggedIn = true
				}
			}

			jobs, err := runHHSearch(page, query, loggedIn, cfg.AutoApply, cfg.Debug)
			if err != nil {
				return err
			}
			b, _ := json.MarshalIndent(jobs, "", "  ")
			fmt.Println(string(b))
			return nil
		}


		if _, err := page.Goto("https://google.com", playwright.PageGotoOptions{Timeout: playwright.Float(30_000)}); err != nil {
			return err
		}
		log.Println("No built-in handler for this task yet.")
		return nil
	}
}



func verifyAndHandleLogin(page playwright.Page, headless bool, profileDir string, autoApply bool) (bool, error) {
	log.Println("Verifying profile login state on hh.ru...")


	urls := []string{"https://hh.ru", "https://hh.ru/applicant"}

	for _, url := range urls {
		if err := navigateWithRetry(page, url, 30_000); err == nil {
			time.Sleep(2 * time.Second)
			clearOverlays(page)

			loggedIn, lerr := isLoggedIn(page)
			if lerr != nil {
				return false, fmt.Errorf("could not determine login state: %w", lerr)
			}

			if loggedIn {
				log.Println("✅ User is already logged in")
				return true, nil
			}
		} else {
			log.Printf("Warning: could not navigate to %s: %v", url, err)
		}
	}


	if headless {
		log.Println("⚠️  Headless режим: проверяю сохраненную сессию...")

		if err := navigateWithRetry(page, "https://hh.ru", 30_000); err == nil {
			time.Sleep(3 * time.Second)
			clearOverlays(page)

			if loggedIn, _ := isLoggedIn(page); loggedIn {
				log.Println("✅ Сессия восстановлена из профиля")
				return true, nil
			}
		}


		log.Println("⚠️  Headless режим: разрешаю работу без логина (только поиск)")
		return false, nil
	}


	log.Println("Profile is not logged-in: opening login page")
	loginURL := "https://hh.ru/account/login"
	if err := navigateWithRetry(page, loginURL, 30_000); err != nil {
		log.Printf("warning: could not open login page %s: %v", loginURL, err)
	}

	if err := page.BringToFront(); err != nil {
		log.Printf("warning: could not bring page to front: %v", err)
	}

	fmt.Println("⏳ Ожидание входа в аккаунт... (5 минут)")
	ok := false
	for i := 0; i < 60; i++ {
		time.Sleep(5 * time.Second)
		clearOverlays(page)
		if l, _ := isLoggedIn(page); l {
			ok = true
			break
		}
		if i%6 == 0 {
			fmt.Printf("   Прошло %d секунд...\n", i*5)
		}
	}
	if !ok {
		return false, fmt.Errorf("login not completed within timeout (5m)")
	}
	log.Println("✅ Login detected — proceeding with task")
	return true, nil
}

func extractQueryFromTask(task string) string {
	stopPhrases := []string{
		"найди", "найти", "ищи", "поищи", "подходящие",
		"подходящих", "подходящую", "подходящие", "подходящая",
		"вакансии", "вакансий", "вакансию", "вакансия",
		"на hh", "на hh.ru", "hh.ru", "hh", "headhunter",
		"3", "4", "5", "несколько", "несколько", "вакансий",
	}

	query := strings.ToLower(task)
	re := regexp.MustCompile(`\d+\s+подходящие\s+`)
	query = re.ReplaceAllString(query, "")
	re = regexp.MustCompile(`\d+\s+подходящих\s+`)
	query = re.ReplaceAllString(query, "")

	for _, phrase := range stopPhrases {
		query = strings.ReplaceAll(query, " "+phrase+" ", " ")
		query = strings.ReplaceAll(query, phrase+" ", " ")
		query = strings.ReplaceAll(query, " "+phrase, " ")
		query = strings.ReplaceAll(query, phrase, " ")
	}

	query = strings.TrimSpace(query)
	query = strings.Trim(query, ".,!?;:-")
	re = regexp.MustCompile(`\s*\.\w{2,}\s*`)
	query = re.ReplaceAllString(query, " ")
	query = strings.TrimSpace(query)
	if query == "" {
		return "AI инженер"
	}

	words := strings.Fields(query)
	for i, word := range words {
		if len(word) > 0 {
			r := []rune(word)
			if len(r) > 0 {
				if (r[0] >= 'а' && r[0] <= 'я') || (r[0] >= 'А' && r[0] <= 'Я') {
					if r[0] >= 'а' && r[0] <= 'я' {
						r[0] = r[0] - ('а' - 'А')
					}
				} else {
					if r[0] >= 'a' && r[0] <= 'z' {
						r[0] = r[0] - ('a' - 'A')
					}
				}
				words[i] = string(r)
			}
		}
	}

	return strings.Join(words, " ")
}

const defaultHHArea = 113

func buildHHSearchURL(query string) string {
	escaped := url.QueryEscape(query)
	return fmt.Sprintf("https://hh.ru/search/vacancy?text=%s&area=%d", escaped, defaultHHArea)
}

func navigateWithRetry(page playwright.Page, url string, timeoutMs int) error {
	const attempts = 3
	var lastErr error
	for i := 1; i <= attempts; i++ {
		log.Printf("navigating to %s (attempt %d/%d)", url, i, attempts)


		if i > 1 {
			time.Sleep(time.Duration(1<<uint(i-1)) * time.Second)
		}

		_, err := page.Goto(url, playwright.PageGotoOptions{
			Timeout:   playwright.Float(float64(timeoutMs)),
			WaitUntil: playwright.WaitUntilStateNetworkidle,
		})
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func clearOverlays(page playwright.Page) {
	selectors := []string{
		"button:has-text(\"Да, верно\")",
		"button:has-text(\"Нет, другой\")",
		"button:has-text(\"Понятно\")",
		"button:has-text(\"Принято\")",
		"button:has-text(\"Принять\")",
		"button:has-text(\"Согласен\")",
		"button:has-text(\"Закрыть\")",
		"button:has-text(\"Нет, спасибо\")",
		"text=Понятно",
		"text=Принято",
		"text=×",
	}

	for _, sel := range selectors {
		loc := page.Locator(sel)
		count, err := loc.Count()
		if err != nil || count == 0 {
			continue
		}
		for i := 0; i < count; i++ {
			el := loc.Nth(i)
			vis, _ := el.IsVisible(playwright.LocatorIsVisibleOptions{Timeout: playwright.Float(60000)})
			if !vis {
				continue
			}
			if err := el.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(60000)}); err != nil {
				log.Printf("clearOverlays: click %s failed: %v", sel, err)
				continue
			}
			log.Printf("clearOverlays: clicked %s", sel)
			time.Sleep(400 * time.Millisecond)
		}
	}

	if err := page.Locator("body").Click(playwright.LocatorClickOptions{Timeout: playwright.Float(10000)}); err == nil {
		time.Sleep(200 * time.Millisecond)
	}
}

func isLoggedIn(page playwright.Page) (bool, error) {
	loginBtn := page.Locator("a:has-text(\"Войти\"), button:has-text(\"Войти\")")
	if c, err := loginBtn.Count(); err == nil && c > 0 {
		for i := 0; i < c; i++ {
			if vis, _ := loginBtn.Nth(i).IsVisible(); vis {
				return false, nil
			}
		}
	}

	profileSelectors := []string{
		"a:has-text(\"Мой профиль\")",
		"a:has-text(\"Профиль\")",
		"img[alt*='аватар']",
		"[data-qa='topbar-account-menu']",
		"[data-qa='main-menu_applicant']",
	}

	for _, sel := range profileSelectors {
		loc := page.Locator(sel)
		if c, err := loc.Count(); err == nil && c > 0 {
			for i := 0; i < c; i++ {
				if vis, _ := loc.Nth(i).IsVisible(); vis {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func findApplyButton(page playwright.Page) (playwright.Locator, error) {
	selectors := []string{
		"button:has-text(\"Откликнуться\")",
		"button:has-text(\"Откликнуться\")",
		"text=Откликнуться",
		"[data-qa='vacancy-response-link-top']",
		"[data-qa='vacancy-serp__vacancy_response']",
	}

	for _, sel := range selectors {
		loc := page.Locator(sel)
		if c, err := loc.Count(); err == nil && c > 0 {
			for i := 0; i < c; i++ {
				el := loc.Nth(i)
				if vis, _ := el.IsVisible(playwright.LocatorIsVisibleOptions{Timeout: playwright.Float(60000)}); vis {
					return el, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("apply button not found")
}

func applyToVacancy(page playwright.Page, message string) (string, error) {
	clearOverlays(page)
	btn, err := findApplyButton(page)
	if err != nil {
		return "", fmt.Errorf("no apply button: %w", err)
	}

	if err := btn.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(60000)}); err != nil {
		return "", fmt.Errorf("could not click apply button: %w", err)
	}

	time.Sleep(1500 * time.Millisecond)

	msgLoc := page.Locator("textarea[name='message'], textarea, input[name='message']")
	if c, _ := msgLoc.Count(); c > 0 {
		for i := 0; i < c; i++ {
			el := msgLoc.Nth(i)
			if vis, _ := el.IsVisible(); vis {
				if err := el.Fill(message); err != nil {
					log.Printf("applyToVacancy: message fill failed: %v", err)
				} else {
					log.Printf("applyToVacancy: filled message field")
				}
				break
			}
		}
	}

	submitSelectors := []string{
		"button:has-text(\"Отправить\")",
		"button:has-text(\"Отправить отклик\")",
		"button:has-text(\"Откликнуться\")",
		"text=Отправить",
		"[data-qa='vacancy-response-submit-popup']",
	}

	for _, sel := range submitSelectors {
		sub := page.Locator(sel)
		if c, _ := sub.Count(); c > 0 {
			for i := 0; i < c; i++ {
				el := sub.Nth(i)
				if vis, _ := el.IsVisible(); !vis {
					continue
				}
				if err := el.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(60000)}); err != nil {
					log.Printf("applyToVacancy: click submit %s failed: %v", sel, err)
					continue
				}

				time.Sleep(2000 * time.Millisecond)

				successPatterns := []string{
					"text=/отправлен|успешно|заявка отправлена|спасибо|выполнено/i",
					":has-text(\"отправлена\")",
					":has-text(\"успешно\")",
					"[data-qa='vacancy-response-popup-success']",
				}

				for _, pattern := range successPatterns {
					if c2, _ := page.Locator(pattern).Count(); c2 > 0 {
						return "submitted successfully", nil
					}
				}

				return "submitted (status confirmation not detected)", nil
			}
		}
	}

	return "no submit action detected", nil
}

func findSearchInput(page playwright.Page) (playwright.Locator, error) {
	clearOverlays(page)
	inputs := page.Locator("input[type='search'], input[type='text'], input:not([type])")
	count, err := inputs.Count()
	if err != nil {
		return nil, fmt.Errorf("could not count inputs: %w", err)
	}

	for i := 0; i < count; i++ {
		in := inputs.Nth(i)
		vis, err := in.IsVisible()
		if err != nil || !vis {
			continue
		}
		ph, err := in.GetAttribute("placeholder")
		if err == nil && ph != "" {
			phLower := strings.ToLower(ph)
			if strings.Contains(phLower, "вакан") || strings.Contains(phLower, "поиск") || strings.Contains(phLower, "должн") {
				return in, nil
			}
		}
	}

	for i := 0; i < count; i++ {
		in := inputs.Nth(i)
		vis, err := in.IsVisible()
		if err != nil && vis {
			return in, nil
		}
	}
	return nil, fmt.Errorf("no visible input found")
}

func runHHSearch(page playwright.Page, query string, loggedIn bool, autoApply bool, debug bool) ([]Job, error) {
	log.Printf("Opening hh.ru and searching for %q\n", query)
	searchURL := buildHHSearchURL(query)
	log.Printf("navigating to search url: %s", searchURL)

	if err := navigateWithRetry(page, searchURL, 60_000); err != nil {
		if debug {
			_ = os.MkdirAll("data/screenshots", 0755)
			if _, serr := page.Screenshot(playwright.PageScreenshotOptions{
				Path:     playwright.String("data/screenshots/hh_failed.png"),
				FullPage: playwright.Bool(true),
			}); serr != nil {
				log.Printf("screenshot failed: %v", serr)
			}
			if html, cerr := page.Content(); cerr == nil {
				_ = os.WriteFile("data/screenshots/hh_failed.html", []byte(html), 0644)
			}
		}
		return nil, fmt.Errorf("goto hh.ru: %w", err)
	}

	time.Sleep(1 * time.Second)
	finalURL := page.URL()
	log.Printf("navigated to final url: %s", finalURL)
	clearOverlays(page)

	input, err := findSearchInput(page)
	if err != nil {
		return nil, fmt.Errorf("find search input: %w", err)
	}

	if err := input.Fill(query); err != nil {
		return nil, fmt.Errorf("fill query: %w", err)
	}
	if err := input.Press("Enter"); err != nil {
		return nil, fmt.Errorf("press enter: %w", err)
	}

	time.Sleep(3 * time.Second)

	anchors := page.Locator("a")
	count, err := anchors.Count()
	if err != nil {
		return nil, fmt.Errorf("count anchors: %w", err)
	}

	type candidate struct {
		href  string
		text  string
		score int
	}
	var cands []candidate

	queryLower := strings.ToLower(query)
	keywords := []string{}

	if strings.Contains(queryLower, "python") {
		keywords = append(keywords, "python", "питон", "django", "flask", "fastapi", "web", "backend")
	}
	if strings.Contains(queryLower, "разработчик") || strings.Contains(queryLower, "developer") {
		keywords = append(keywords, "разработчик", "developer", "программист", "engineer", "инженер")
	}
	if strings.Contains(queryLower, "java") {
		keywords = append(keywords, "java", "джава", "spring", "hibernate")
	}
	if strings.Contains(queryLower, "javascript") || strings.Contains(queryLower, "js") {
		keywords = append(keywords, "javascript", "js", "react", "angular", "vue", "node", "frontend")
	}

	if len(keywords) == 0 {
		words := strings.Fields(queryLower)
		keywords = append(keywords, words...)
	}

	excludeKeywords := []string{
		"кассир", "водитель", "логист", "продавец", "охран",
		"грузчик", "монтер", "электрик", "сварщик", "механик",
		"маляр", "уборщик", "повар", "шеф", "официант",
		"администратор гости", "администратор отел",
	}

	for i := 0; i < count; i++ {
		a := anchors.Nth(i)
		vis, err := a.IsVisible()
		if err != nil || !vis {
			continue
		}

		text, err := a.TextContent()
		if err != nil || text == "" {
			continue
		}

		href, err := a.GetAttribute("href")
		if err != nil || href == "" {
			continue
		}

		lt := strings.ToLower(text)
		isExcluded := false
		for _, excl := range excludeKeywords {
			if strings.Contains(lt, excl) {
				isExcluded = true
				break
			}
		}
		if isExcluded {
			continue
		}

		score := 0
		hasKeyword := false

		for _, kw := range keywords {
			if strings.Contains(lt, kw) {
				score += 5
				hasKeyword = true
			}
		}

		if !hasKeyword {
			continue
		}

		if strings.Contains(href, "vacancy") || strings.Contains(href, "vacancies") || strings.Contains(href, "vacans") {
			score += 2
		}

		if score >= 5 {
			cands = append(cands, candidate{
				href:  href,
				text:  strings.TrimSpace(text),
				score: score,
			})
		}
	}

	if len(cands) == 0 {
		log.Println("no candidates found via heuristics, collecting top anchors as fallback")
		for i := 0; i < count && i < 30; i++ {
			a := anchors.Nth(i)
			vis, err := a.IsVisible()
			if err != nil || !vis {
				continue
			}

			text, err := a.TextContent()
			if err != nil || text == "" {
				continue
			}

			href, err := a.GetAttribute("href")
			if err != nil || href == "" {
				continue
			}

			cands = append(cands, candidate{
				href:  href,
				text:  strings.TrimSpace(text),
				score: 1,
			})
		}
	}

	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	seen := map[string]struct{}{}
	jobs := []Job{}

	for _, c := range cands {
		if len(jobs) >= 3 {
			break
		}

		h := c.href
		if strings.HasPrefix(h, "/") {
			h = "https://hh.ru" + h
		}

		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}

		p, err := page.Context().NewPage()
		if err != nil {
			log.Printf("failed to create new page: %v", err)
			continue
		}

		if err := navigateWithRetry(p, h, 90_000); err != nil {
			log.Printf("failed to goto %s: %v", h, err)
			p.Close()
			continue
		}

		time.Sleep(800 * time.Millisecond)
		clearOverlays(p)

		title := c.text
		if t, err := p.Locator("h1").First().TextContent(); err == nil && t != "" {
			title = strings.TrimSpace(t)
		}

		company := ""
		if ctext, err := p.Locator("[data-qa='vacancy-company-name']").First().TextContent(); err == nil && ctext != "" {
			company = strings.TrimSpace(ctext)
		}

		if company == "" {
			divs := p.Locator("div")
			divCount, _ := divs.Count()
			for i := 0; i < divCount; i++ {
				n := divs.Nth(i)
				txt, err := n.TextContent()
				if err == nil && txt != "" {
					txtLower := strings.ToLower(txt)
					if strings.Contains(txtLower, "компан") || strings.Contains(txtLower, "ооо") || strings.Contains(txt, "—") {
						company = strings.TrimSpace(txt)
						break
					}
				}
			}
		}

		snippet := ""
		descSelectors := []string{
			"[data-qa='vacancy-description']",
			"[data-qa='skills-table']",
			"div[class*='description']",
			"article",
		}

		for _, sel := range descSelectors {
			descLoc := p.Locator(sel).First()
			if txt, err := descLoc.TextContent(); err == nil && txt != "" {
				trimmed := strings.TrimSpace(txt)
				if len(trimmed) > 500 {
					snippet = trimmed[:500] + "..."
				} else {
					snippet = trimmed
				}
				break
			}
		}

		if snippet == "" {
			bodyLocator := p.Locator("body")
			if body, err := bodyLocator.TextContent(); err == nil && body != "" {
				trimmed := strings.TrimSpace(body)
				if len(trimmed) > 300 {
					snippet = trimmed[:300] + "..."
				} else {
					snippet = trimmed
				}
			}
		}

		job := Job{
			Title:   title,
			Company: company,
			URL:     h,
			Snippet: snippet,
		}

		if loggedIn && autoApply {
			res, aerr := applyToVacancy(p, "Здравствуйте, заинтересован(а) в этой вакансии. Можете связаться со мной для обсуждения.")
			if aerr != nil {
				log.Printf("applyToVacancy failed for %s: %v", h, aerr)
				job.Applied = false
				job.ApplyResult = aerr.Error()
			} else {
				job.Applied = true
				job.ApplyResult = res
				log.Printf("Applied to %s: %s", h, res)
			}
		}

		jobs = append(jobs, job)
		p.Close()
	}

	return jobs, nil
}

func GetJob(title, company, url, snippet string) Job {
	return types.GetJob(title, company, url, snippet)
}


func isRealVacancyURL(url string) bool {
	if url == "" {
		return false
	}

	urlLower := strings.ToLower(url)

	mustContainPatterns := []string{
		"/vacancy/",
		"vacancy/",
	}


	excludePatterns := []string{
		"/article/",
		"/special/",
		"/partner/",
		"/promo/",
		"?hhtmfrom=",
		"utm_source=",
		"utm_campaign=",
		"lead_gen",
		"gazpromneft",
		"hhtmpromo",
		"from=share_ios",
	}


	hasVacancy := false
	for _, pattern := range mustContainPatterns {
		if strings.Contains(urlLower, pattern) {
			hasVacancy = true
			break
		}
	}

	if !hasVacancy {
		return false
	}


	for _, pattern := range excludePatterns {
		if strings.Contains(urlLower, pattern) {
			return false
		}
	}


	if urlLower == "https://hh.ru" ||
		urlLower == "https://hh.ru/" ||
		strings.Contains(urlLower, "hh.ru/?") ||
		strings.Contains(urlLower, "hh.ru#") {
		return false
	}


	re := regexp.MustCompile(`/vacancy/(\d+)`)
	matches := re.FindStringSubmatch(urlLower)
	if len(matches) < 2 {
		return false 
	}


	vacancyID := matches[1]
	if _, err := strconv.Atoi(vacancyID); err != nil {
		return false
	}

	return true
}


func normalizeVacancyURL(url string) string {
	if url == "" {
		return url
	}


	if idx := strings.Index(url, "?"); idx != -1 {
		base := url[:idx]


		query := url[idx+1:]
		params := strings.Split(query, "&")

		var keepParams []string
		for _, param := range params {
			if strings.HasPrefix(param, "query=") ||
				strings.HasPrefix(param, "from=") ||
				strings.HasPrefix(param, "hhtmFromLabel=") {
				keepParams = append(keepParams, param)
			}
		}

		if len(keepParams) > 0 {
			return base + "?" + strings.Join(keepParams, "&")
		}
		return base
	}


	if strings.HasPrefix(url, "//") {
		return "https:" + url
	}


	if strings.HasPrefix(url, "/") {
		return "https://hh.ru" + url
	}

	return url
}


func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
