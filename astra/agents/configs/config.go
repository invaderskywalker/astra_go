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
	data, err := os.ReadFile("astra/agents/configs/agents/astra.yaml")
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

// ActionYAMLConfig for loading description/details from YAML
type ActionYAMLConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Details     string `yaml:"details"`
}

// loadLearningActionYAML loads a specific YAML config by filename
func loadLearningActionYAML(filename string) (*ActionYAMLConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var cfg ActionYAMLConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadActionsYAMLInDir(dir string) (map[string]*ActionYAMLConfig, error) {
	result := make(map[string]*ActionYAMLConfig)
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		cfg, err := loadLearningActionYAML(f)
		if err == nil && cfg.Name != "" {
			result[cfg.Name] = cfg
		}
	}
	return result, nil
}
