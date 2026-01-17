package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/mihnpro/Ai_agent_2/internal/browser"
    "github.com/mihnpro/Ai_agent_2/internal/types"
    "gopkg.in/yaml.v3"
)


func loadConfig() (types.Config, error) {
    cfg := types.Config{
        Headless:   true,
        Debug:      false,
        UseAI:      true,
        AutoApply:  false,
        ProfileDir: filepath.Join(os.Getenv("HOME"), ".ai-job-agent", "profile"),
        ModelURL:   "http://localhost:11434",
        ModelName:  "qwen2.5:7b",
        MaxJobs:    3,
    }

    data, err := os.ReadFile("config.yaml")
    if err != nil {
        log.Printf("Using default config: %v", err)
        return cfg, nil
    }

    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return cfg, fmt.Errorf("parse config: %w", err)
    }

    return cfg, nil
}

func main() {
    var task string
    var headless bool
    var debug bool
    var autoApply bool
    var profileDir string
    var modelURL string
    var modelName string
    var interactive bool

    flag.StringVar(&task, "task", "", "Task for the agent")
    flag.BoolVar(&headless, "headless", true, "Run browser in headless mode")
    flag.BoolVar(&debug, "debug", false, "Enable debug mode")
    flag.BoolVar(&autoApply, "auto-apply", false, "Auto apply to suitable jobs")
    flag.StringVar(&profileDir, "profile", filepath.Join(os.Getenv("HOME"), ".ai-job-agent", "profile"), "Browser profile directory")
    flag.StringVar(&modelURL, "model-url", "http://localhost:11434", "Ollama model URL")
    flag.StringVar(&modelName, "model", "qwen2.5:7b", "Model name for Ollama")
    flag.BoolVar(&interactive, "interactive", false, "Interactive mode")
    flag.Parse()

    cfg, err := loadConfig()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    if flag.Parsed() {
        cfg.Headless = headless
        cfg.Debug = debug
        cfg.AutoApply = autoApply
        if profileDir != "" {
            cfg.ProfileDir = profileDir
        }
        if modelURL != "" {
            cfg.ModelURL = modelURL
        }
        if modelName != "" {
            cfg.ModelName = modelName
        }
    }

    if interactive {
        runInteractiveMode(cfg)
        return
    }

    if task == "" {
        if len(flag.Args()) > 0 {
            task = flag.Args()[0]
        } else {
            fmt.Println("Enter task for agent:")
            fmt.Scanln(&task)
            if task == "" {
                task = "Найди 3 подходящие вакансии AI-инженера на hh.ru"
            }
        }
    }

    fmt.Printf("🤖 AI Job Agent Starting...\n")
    fmt.Printf("   Task: %s\n", task)
    fmt.Printf("   Model: %s at %s\n", cfg.ModelName, cfg.ModelURL)
    fmt.Printf("   Profile: %s\n", cfg.ProfileDir)
    fmt.Printf("   Mode: %s\n", func() string {
        if cfg.Headless {
            return "headless"
        }
        return "visible"
    }())
    if cfg.AutoApply {
        fmt.Printf("   ⚠️  AUTO-APPLY ENABLED\n")
    }

    if err := browser.RunTask(task, cfg); err != nil {
        log.Fatalf("❌ Agent failed: %v", err)
    }

    fmt.Println("\n✅ Agent completed successfully!")
}

func runInteractiveMode(cfg types.Config) {
    fmt.Println("🤖 AI Job Agent - Interactive Mode")
    fmt.Println("Type 'quit' or 'exit' to end")
    fmt.Println()

    for {
        fmt.Print("> ")
        var input string
        fmt.Scanln(&input)

        if input == "quit" || input == "exit" {
            fmt.Println("Goodbye!")
            return
        }

        if input != "" {
            fmt.Printf("Executing: %s\n", input)
            if err := browser.RunTask(input, cfg); err != nil {
                fmt.Printf("Error: %v\n", err)
            }
        }
    }
}