package configs

import (
	"astra/astra/utils/logging"
	"os"
	"path/filepath"

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
	AgentName       string                `yaml:"agent_name"`
	AgentRole       string                `yaml:"agent_role"`
	DecisionProcess DecisionProcessConfig `yaml:"decision_process"`
	OutputFormats   OutputFormats         `yaml:"output_formats"`
}

// ---------- LOADER ----------

func LoadConfig() *AgentConfig {
	// Try local relative path first (works in dev)
	localPath := "astra/agents/configs/agents/astra.yaml"
	var yamlPath string

	if _, err := os.Stat(localPath); err == nil {
		yamlPath = localPath
	} else {
		// Fallback for global binary (/usr/local/bin/astra)
		home, _ := os.UserHomeDir()
		yamlPath = filepath.Join(home, "Documents", "projects", "llm_apps", "astra_go", "astra", "agents", "configs", "agents", "astra.yaml")
		if _, err := os.Stat(yamlPath); err != nil {
			logging.ErrorLogger.Error("❌ Could not find astra.yaml config file", zap.String("checked_path", yamlPath))
			return &AgentConfig{}
		}
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		logging.ErrorLogger.Error("Failed to read config YAML", zap.Error(err))
		return &AgentConfig{}
	}

	cfg := &AgentConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		logging.ErrorLogger.Error("Failed to parse config YAML", zap.Error(err))
		return &AgentConfig{}
	}

	logging.AppLogger.Info("📘 Loaded Agent Config", zap.String("path", yamlPath))
	return cfg
}
