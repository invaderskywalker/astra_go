package configs

import (
	"astra/astra/utils/logging"
	"fmt"
	"os"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	AgentName              string            `yaml:"agent_name"`
	AgentRole              string            `yaml:"agent_role"`
	DecisionProcess        string            `yaml:"decision_process"`
	UserIntentsClasses     map[string]string `yaml:"user_intents_classes"`
	AvailableDataSources   []string          `yaml:"available_data_sources"`
	AvailableActions       []string          `yaml:"available_actions"`
	UserCTAInstructions    string            `yaml:"user_cta_instructions"`
	PlanOutputStructure    string            `yaml:"llm1_plan_output_structure"`
	EditSchemaInstructions string            `yaml:"edit_schema_instructions"`
	AdditionalInfo         string            `yaml:"additional_info"`
}

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

	fmt.Printf("cfg loaded: %+v\n", cfg) // Debug print

	return cfg
}
