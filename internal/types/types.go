package types

type Job struct {
	Title       string `json:"title"`
	Company     string `json:"company"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	Applied     bool   `json:"applied"`
	ApplyResult string `json:"apply_result,omitempty"`
	MatchScore  int    `json:"match_score,omitempty"`
}

type Config struct {
	Headless   bool
	Debug      bool
	UseAI      bool
	AutoApply  bool
	ProfileDir string
	ModelURL   string
	ModelName  string
	MaxJobs    int
}

type Resume struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Experience  []string `json:"experience"`
	Skills      []string `json:"skills"`
	Education   []string `json:"education"`
	Summary     string   `json:"summary"`
	ContactInfo Contact  `json:"contact_info"`
}

type Contact struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	LinkedIn string `json:"linkedin"`
}

type AgentAction struct {
	Action     string                 `json:"action"`
	Parameters map[string]interface{} `json:"parameters"`
	Reason     string                 `json:"reason"`
	Result     string                 `json:"result,omitempty"`
	Timestamp  string                 `json:"timestamp"`
}

func GetJob(title, company, url, snippet string) Job {
	return Job{
		Title:   title,
		Company: company,
		URL:     url,
		Snippet: snippet,
	}
}
