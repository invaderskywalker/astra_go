// astra/agents/configs/config.go
package configs

import (
	"encoding/json"
	"os"
)

type AgentConfig struct {
	Name             string            `json:"agent_name"`
	Role             string            `json:"agent_role"`                 // e.g., "dev", "advisor", "friend"
	Responsibilities []string          `json:"responsibilities"`           // e.g., ["modify codebase", "replicate DB", "web search"]
	AvailableGetters []string          `json:"available_data_sources"`     // e.g., ["summarize_codebase", "fetch_file_content"]
	AvailableActions []string          `json:"available_actions"`          // e.g., ["apply_code_edits", "switch_git_branch"]
	UserIntents      map[string]string `json:"user_intents_classes"`       // e.g., {"modify_code": "Plan edits with schema"}
	PlanningPrompt   string            `json:"llm1_plan_output_structure"` // JSON schema for plan
	ResponsePrompt   string            `json:"response_prompt_template"`
	EditSchema       string            `json:"edit_schema_instructions"` // Embed simplified schema
	DecisionProcess  string            `json:"decision_process"`
	AdditionalInfo   string            `json:"additional_info"`
	WebAPIKey        string            `json:"web_api_key,omitempty"` // For later web integration
}

func LoadConfig() *AgentConfig {
	fileBytes, err := os.ReadFile("astra/agents/configs/agent.json")
	if err != nil {
		// logging.Logger.Error("Config load error", "error", err)
		return &AgentConfig{} // Default fallback
	}

	var cfg AgentConfig
	if err := json.Unmarshal(fileBytes, &cfg); err != nil {
		// logging.Logger.Error("Config unmarshal error", "error", err)
		return &AgentConfig{}
	}
	return &cfg
}
