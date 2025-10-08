// Package actions provides functionality for managing database and code manipulation actions.
package actions

import (
	"encoding/json"
	"fmt"
	"reflect"

	"gorm.io/gorm"
)

// DataActions manages a set of actions for database and code manipulation.
type DataActions struct {
	actions map[string]ActionSpec
	db      *gorm.DB
}

type ActionSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ActionSpec describes metadata for a registered action.
type ActionSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Params      interface{} `json:"params"`   // Struct type for parameters
	Returns     interface{} `json:"returns"`  // Struct type for return
	Examples    []string    `json:"examples"` // Usage examples (JSON or text)
	Fn          interface{} `json:"-"`        // Actual function (not serialized)
}

// NewDataActions initializes the DataActions registry.
func NewDataActions(db *gorm.DB) *DataActions {

	a := &DataActions{
		actions: make(map[string]ActionSpec),
		db:      db,
	}

	a.register(ActionSpec{
		Name:        "apply_code_edits",
		Description: "Applies a list of code edits (replace, insert, create_file, delete_file) to source files. Only to be triggered when code edit is required. Always see this file for examples on how to add. code_edits_test",
		Params:      ApplyCodeEditsParams{},
		// Returns:     ApplyCodeEditsResult{},
		Examples: []string{
			`{
				"edits": [
					{
						"type": "replace",
						"target": "return None",
						"context_before": "def bar(",
						"replacement": "return 'ok'",
						"file": "path",
					}
				]
			}`,
			`{
				"edits": [
					{
						"type": "insert",
						"target": "class UserController:",
						"position": "after",
						"content": "    def deactivate_user(self, user_id: int):\n        pass\n",
						"file": "path",
					}
				]
			}`,
		},
		Fn: a.applyCodeEdits,
	})

	a.register(ActionSpec{
		Name:        "fetch_file_structure_in_this_repo",
		Description: "Fetches the file and folder structure of the current repository using the system `tree` command.",
		Params:      FetchFileStructureParams{},
		// Returns:     FetchFileStructureResult{},
		Examples: []string{
			`{
				"path": ".",
				"ignore_dirs": [".git", ".vscode", "logs"]
			}`,
		},
		Fn: a.FetchFileStructureInRepo,
	})

	a.register(ActionSpec{
		Name:        "ask_follow_up_questions_to_user",
		Description: "Takes an array of questions and returns a structured list with placeholder answers. Can be extended to use LLM reasoning later.",
		Params:      AskFollowUpQuestionsParams{},
		// Returns:     AskFollowUpQuestionsResult{},
		Examples: []string{
			`{
				"questions": [
					"Q1",
					"Q2",
				]
			}`,
		},
		Fn: a.AskFollowUpQuestions,
	})

	a.register(ActionSpec{
		Name:        "read_files_in_this_repo",
		Description: "Reads the content of multiple files from the current repository safely.",
		Params:      ReadFilesParams{},
		Examples: []string{
			`{
				"paths": [
					"astra/agents/actions/actions.go",
					"astra/agents/core/agent.go"
				]
			}`,
		},
		Fn: a.ReadFilesInRepo,
	})

	a.register(ActionSpec{
		Name:        "scrape_urls",
		Description: "Scrapes given URLs and returns their text content using Playwright.",
		Params:      ScrapeURLsParams{},
		Returns:     ScrapeURLsResult{},
		Examples: []string{
			`{"urls": ["https://example.com", "https://wikipedia.org"]}`,
		},
		Fn: a.ScrapeURLs,
	})

	a.register(ActionSpec{
		Name:        "query_web",
		Description: "Performs a web search for the given queries and returns text snippets (DuckDuckGo or similar).",
		Params:      QueryWebParams{},
		Returns:     QueryWebResult{},
		Examples: []string{
			`{"queries": ["best golang web scraper",...], "result_limit": 3}`,
		},
		Fn: a.QueryWeb,
	})

	return a
}

// register adds an action spec to the registry.
func (a *DataActions) register(spec ActionSpec) {
	a.actions[spec.Name] = spec
}

// ListActions returns all registered action metadata (excluding function pointers).
func (a *DataActions) ListActions() []ActionSpec {
	specs := make([]ActionSpec, 0, len(a.actions))
	for _, spec := range a.actions {
		specs = append(specs, spec)
	}
	return specs
}

// GetAction retrieves a specific ActionSpec by name.
func (a *DataActions) GetAction(name string) (ActionSpec, bool) {
	spec, ok := a.actions[name]
	return spec, ok
}

func (a *DataActions) ListActionSummaries() []ActionSummary {
	summaries := make([]ActionSummary, 0, len(a.actions))
	for _, spec := range a.actions {
		summaries = append(summaries, ActionSummary{
			Name:        spec.Name,
			Description: spec.Description,
		})
	}
	return summaries
}

// ExecuteAction executes a registered action by name using the provided params (map).
// It returns the action's result as a map[string]interface{} or an error.
func (a *DataActions) ExecuteAction(name string, rawParams map[string]interface{}) (map[string]interface{}, error) {
	spec, ok := a.actions[name]
	if !ok {
		return nil, fmt.Errorf("action not found: %s", name)
	}
	if spec.Fn == nil {
		return nil, fmt.Errorf("no function registered for action: %s", name)
	}

	fnVal := reflect.ValueOf(spec.Fn)
	fnType := fnVal.Type()

	// Currently we only support functions with exactly 1 input parameter.
	numIn := fnType.NumIn()
	var argVal reflect.Value
	if numIn == 0 {
		// no input; nothing to prepare
	} else if numIn == 1 {
		inType := fnType.In(0)

		// Create a pointer to the expected input type so we can json.Unmarshal into it
		inPtr := reflect.New(inType)
		// But json.Unmarshal expects a pointer to a concrete type. We'll marshal rawParams and unmarshal.
		paramBytes, err := json.Marshal(rawParams)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		if err := json.Unmarshal(paramBytes, inPtr.Interface()); err != nil {
			return nil, fmt.Errorf("failed to unmarshal params into %s: %w", inType.String(), err)
		}

		// If function expects a non-pointer (value), pass Elem(); else pass pointer
		if inType.Kind() == reflect.Ptr {
			argVal = inPtr
		} else {
			argVal = inPtr.Elem()
		}
	} else {
		return nil, fmt.Errorf("action function has unsupported number of input parameters: %d", numIn)
	}

	// Build arguments slice
	var args []reflect.Value
	if numIn == 1 {
		args = []reflect.Value{argVal}
	} else {
		args = []reflect.Value{}
	}

	// Call the function
	outVals := fnVal.Call(args)

	// Handle outputs:
	// supported forms:
	//  - (ResultStruct)
	//  - (ResultStruct, error)
	if len(outVals) == 0 {
		return nil, nil
	}

	// If second return is error, check it
	if len(outVals) == 2 {
		// second value should be error or nil
		errInterface := outVals[1].Interface()
		if errInterface != nil {
			if errObj, ok := errInterface.(error); ok {
				return nil, errObj
			}
			// not standard error type, format it
			return nil, fmt.Errorf("action returned non-nil second value: %v", errInterface)
		}
		// proceed to convert first output to map
	}

	// Convert first return value to map[string]interface{}
	first := outVals[0].Interface()
	// marshal then unmarshal into map for simplicity
	b, err := json.Marshal(first)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal action result: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		// If result isn't a JSON object (could be a primitive), wrap it
		return map[string]interface{}{"result": first}, nil
	}
	return result, nil
}
