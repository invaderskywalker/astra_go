package configs

import (
	"astra/astra/utils/logging"
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// ---------- STRUCTS ----------

// DecisionProcessConfig represents the whole decision pipeline
type DecisionProcessConfig struct {
	Description string `yaml:"description"`
	// Stages      []DecisionStep `yaml:"stages"`
}

type DecisionStep struct {
	Name     string `yaml:"name"`
	Purpose  string `yaml:"purpose"`
	Behavior string `yaml:"behavior"`
	// Outputs  []map[string]interface{} `yaml:"outputs"`
}

// OutputFormats holds JSON schema templates
type OutputFormats struct {
	PlanOutputJSON          string `yaml:"plan_output_json"`
	ExecutionStepOutputJSON string `yaml:"execution_step_output_json"`
	FinalSummaryJSON        string `yaml:"final_summary_json"`
}

// AgentConfig matches astra.yaml
type AgentConfig struct {
	AgentName        string                `yaml:"agent_name"`
	AgentRole        string                `yaml:"agent_role"`
	DecisionProcess  DecisionProcessConfig `yaml:"decision_process"`
	AvailableActions []string              `yaml:"available_actions"`
	OutputFormats    OutputFormats         `yaml:"output_formats"`
}

// ---------- LOADER ----------

func LoadConfig() *AgentConfig {
	data, err := os.ReadFile("astra/agents/configs/astra.yaml")
	if err != nil {
		logging.ErrorLogger.Error("Failed to read config YAML", zap.Error(err))
		return &AgentConfig{}
	}

	cfg := &AgentConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		logging.ErrorLogger.Error("Failed to parse config YAML", zap.Error(err))
		return &AgentConfig{}
	}

	return cfg
}
