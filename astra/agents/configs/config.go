package configs

import (
	"astra/astra/utils/logging"
	"strings"

	"github.com/magiconair/properties"
	"go.uber.org/zap"
)

type AgentConfig struct {
	AgentName              string
	AgentRole              string
	DecisionProcess        string
	UserIntentsClasses     map[string]string
	AvailableDataSources   []string
	AvailableActions       []string
	UserCTAInstructions    string
	PlanningPrompt         string
	EditSchemaInstructions string
	AdditionalInfo         string
}

func LoadConfig() *AgentConfig {
	props, err := properties.LoadFile("astra/agents/configs/astra.properties", properties.UTF8)
	if err != nil {
		logging.AppLogger.Error("Config load error", zap.Error(err))
		return &AgentConfig{}
	}

	// helper to parse comma-separated values
	parseSlice := func(val string) []string {
		if val == "" {
			return []string{}
		}
		parts := strings.Split(val, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}

	cfg := &AgentConfig{
		AgentName:              props.GetString("agent_name", "Astra"),
		AgentRole:              props.GetString("agent_role", ""),
		DecisionProcess:        props.GetString("decision_process", ""),
		UserIntentsClasses:     make(map[string]string),
		AvailableDataSources:   parseSlice(props.GetString("available_data_sources", "")),
		AvailableActions:       parseSlice(props.GetString("available_actions", "")),
		UserCTAInstructions:    props.GetString("user_cta_instructions", ""),
		PlanningPrompt:         props.GetString("llm1_plan_output_structure", ""),
		EditSchemaInstructions: props.GetString("edit_schema_instructions", ""),
		AdditionalInfo:         props.GetString("additional_info", ""),
	}

	// Populate UserIntentsClasses
	intents := []string{
		"conversational",
		"code_edit",
		"db_replication",
		"repo_management",
		"user_submit_onboarding_info",
		"clarification_needed",
	}

	for _, intent := range intents {
		key := "user_intents_classes_" + intent
		value := props.GetString(key, "")
		if value != "" {
			cfg.UserIntentsClasses[intent] = value
		}
	}

	return cfg
}
