// Package actions provides functionality for managing database and code manipulation actions.

package actions

import (
	"astra/astra/agents/configs"
	"astra/astra/sources/psql/dao"
	"encoding/json"
	"fmt"
	"reflect"

	"gorm.io/gorm"
)

// DataActions manages a set of actions for database and code manipulation.
type DataActions struct {
	actions              map[string]ActionSpec
	db                   *gorm.DB
	UserID               int
	longTermKnowledgeDao *dao.LongTermKnowledgeDAO
}

type ActionSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ActionSpec describes metadata for a registered action.
type ActionSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Details     string      `json:"details"`
	Params      interface{} `json:"params"` // Struct type for parameters
	Fn          interface{} `json:"-"`      // Actual function (not serialized)
}

// NewDataActions initializes the DataActions registry.
func NewDataActions(db *gorm.DB, userId int) *DataActions {
	longTermKnowledgeDao := dao.NewLongTermKnowledgeDAO(db)
	a := &DataActions{
		actions:              make(map[string]ActionSpec),
		db:                   db,
		UserID:               userId,
		longTermKnowledgeDao: longTermKnowledgeDao,
	}

	a.register(ActionSpec{
		Name: "apply_code_edits",
		Description: `
		Applies intelligent, context-aware code modifications to source files
		within the Astra repository. Supports:
		  • insert
		  • replace
		  • create_file
		  • delete_file
		  • replace_file  ← replaces an entire file atomically.
		Use only when the agent must perform a real code update, never for text generation.
	`,
		Details: `
		# 🧠 Astra Code Editing Engine

		The Astra Code Editing Engine enables safe, structured, and context-aware
		source-code modifications inside the Astra Agent framework.

		---

		## ⚙️ Core Files
		• Implementation: astra/agents/actions/code_edits.go  
		• Tests:          astra/agents/actions/code_edits_test.go

		---

		## 🧩 Core Data Structures
		Each code edit is represented as a JSON object:

		- type: "insert" | "replace" | "create_file" | "delete_file" | "replace_file"
		- file: path to the target file (required)

		### insert
		Insert new lines before/after a target.
		Fields:
		  • target: reference line to locate to insert before or after as posiiton says 
		  • position: "before" | "after" (default "after")
		  • content: new content for inserts or new files
		  • - context_before / context_after: lines near target for safe matching (if multiple places have same target line)

		### replace
		Update an existing line or block.
		Fields:
		  • start / end: block boundaries (this is not line number, this is code line be very mindful of this) (for replace) (must have)
		  • replacement: content to replace existing code (must have)
		  • position: "before" | "after" (default "after")
		  • context_before / context_after: lines near target for safe matching (if multiple places have same start/end block)

		### create_file
		Create a new file with the given content.

		### delete_file
		Delete a file if it exists.

		### replace_file  ← **New Ability**
		Atomically replace the entire contents of a file with the given text.
		Use when performing large rewrites or regenerating code.
		Skips all target/search logic and directly overwrites the file.

		---
			## Important

			__BOF__ and __EOF__ are also supported for targets
			You should use insert if writing a new code (like new line(s)/ new functinos, new classes)
			Use replace mostly if you are trying to update some line(s) of code with other block

			While giving a json output never use backticks. always ouptut proper JSON

		----
		Example:
		{
			"edits": [
				{
					"type": "replace_file",
					"file": "astra/agents/actions/actions.go",
					"replacement": "// full new contents of the file\npackage actions\n\n..."
				}
			]
		}

		---

		## 🧠 Execution Flow
		1. Validate and group edits by file.
		2. Apply each edit type sequentially:
		     create_file → new file
		     delete_file → remove file
		     replace_file → full overwrite (atomic)
		     insert / replace → partial modifications
		3. Run go fmt automatically after edits.

		---

		## 💡 LLM Prompt Guidelines
		- Preserve indentation exactly.
		- Never use backticks in JSON output.
		- Limit edits to the specified region (or whole file for replace_file).
		- Always provide a valid file path.
		- For risky edits, call think_aloud_reasoning first.

		---

		**In essence:**  
		The Astra Code Editing Engine gives agents the power to modify or regenerate
		any part of the codebase—safely, predictably, and under precise control.
	`,
		Params: ApplyCodeEditsParams{},
		Fn:     a.applyCodeEdits,
	})

	a.register(ActionSpec{
		Name:        "fetch_file_structure_in_this_repo",
		Description: "Fetches the file and folder structure of the current repository using the system `tree` command.",
		Details: `Usage Exampole: {
				"path": ".",
				"ignore_dirs": [".git", ".vscode", "logs"]
			} `,
		Params: FetchFileStructureParams{},
		Fn:     a.FetchFileStructureInRepo,
	})

	a.register(ActionSpec{
		Name:        "ask_follow_up_questions_to_user",
		Description: "This is created to initiate asking questions to user.",
		Details: `Usage Example: {
				"questions": [
					"Q1",
					"Q2",
				]
			}`,
		Params: AskFollowUpQuestionsParams{},
		Fn:     a.AskFollowUpQuestions,
	})

	a.register(ActionSpec{
		Name:        "read_files_in_this_repo",
		Description: "Reads the content of multiple files from the current repository safely.",
		Details: `Usage example- {
				"paths": [
					"astra/agents/actions/actions.go",
					"astra/agents/core/agent.go"
				]
			}`,
		Params: ReadFilesParams{},
		Fn:     a.ReadFilesInRepo,
	})

	a.register(ActionSpec{
		Name:        "scrape_urls",
		Description: "Scrapes given URLs and returns clean readable text content from each page using Playwright browser automation.",
		Details: `
			# 🌐 Astra Web Scraping Action

					The "scrape_urls" action launches a headless Playwright browser 
					to scrape multiple web pages concurrently. 
					It extracts readable text while skipping ads, scripts, and media resources.

			---


			## 🧩 Input 

					Input → ScrapeURLsParams
						- urls: []string → list of URLs to scrape

					

			---

			## 🚀 Example Usage

					{
						"urls": [
							"https://example.com",
							"https://wikipedia.org"
						],
						"word_limit": 10000,
					}

			---

			**In essence:**
					This action allows Astra to browse and extract text content from the web,
					serving as the foundation for knowledge retrieval, summarization, 
					or data enrichment workflows.
	`,
		Params: ScrapeURLsParams{},
		Fn:     a.ScrapeURLs,
	})

	a.register(ActionSpec{
		Name:        "query_web",
		Description: "Performs a search query on DuckDuckGo and returns top search results (titles, snippets, and links).",
		Details: `
			# 🔍 Astra Web Query Action

					The "query_web" action performs a fast, privacy-friendly web search 
					using DuckDuckGo HTML interface. 
					It extracts titles, snippets, and actual destination URLs.

			---

			## 🧩 Input / Output

					Input → QueryWebParams
						- queries: []string → search phrases
						- result_limit: int → number of results per query

					Output → QueryWebResult
						- results: map[string]interface{} → each query key maps to list of:
								• url     → actual resolved URL
								• title   → search result title
								• snippet → short text summary

			---

			## 🚀 Example Usage

					{
						"queries": ["golang concurrency", "ai autonomous agents"],
						"result_limit": 3
					}

			---

			**In essence:**
					This action gives Astra the ability to perform real-time web lookups 
					and retrieve contextual search snippets for reasoning or LLM grounding.
	`,
		Params: QueryWebParams{},
		Fn:     a.QueryWeb,
	})

	a.register(ActionSpec{
		Name:        "fmt_vet_build",
		Description: "Formats (go fmt), vets (go vet), and builds (go build) the entire Go project. Used to validate Astra’s code after edits.",
		Details: `
			# 🧹 Astra Code Validation Action

			This action ensures that Astra’s Go codebase is correctly formatted,
			static analysis passes, and the project compiles without errors.

			- Runs: go fmt ./...
			- Runs: go vet ./...
			- Runs: go build ./...

			**Usage Example**
			{
				"action": "fmt_vet_build",
				"action_params": {}
			}
		`,
		Params: struct{}{}, // no params needed
		Fn:     a.FmtVetBuild,
	})

	a.register(ActionSpec{
		Name:        "pwd",
		Description: "Fetch current working directory",
		Details:     ``,
		Params:      struct{}{},
		Fn:          a.GetPWD,
	})

	a.register(ActionSpec{
		Name:        "think_aloud_reasoning",
		Description: "Use this when you are about to make some important changes, before performing risky actions such as code edits or critical logic updates, because correct reasoning is really important.",
		Details: `
			# 🧠 Astra Think-Aloud Reasoning Action

			This special action triggers Astra's internal reasoning stream.

			When Astra decides to call this action in its plan, it pauses to *think aloud* —  
			streaming its internal thought process using the same LLM client.

			The reasoning can include:
			- Why a certain code edit or step is being considered.
			- What potential effects or risks it might have.
			- Final summarized decision under "FINAL THOUGHT:".

			It does not perform any edit or side-effect itself.
			The action is meant purely for deep reasoning visibility before a risky or uncertain operation.

			Examples::
			{
				"action": "think_aloud_reasoning",
				"action_params": {
					"context": "", // important ot pass context and goal
					"goal": ""
				}
			}
		`,
		Params: struct{}{}, // no params required, but agent may internally provide context
		Fn:     nil,        // intentionally nil — handled internally in BaseAgent
	})

	// --- Register learning actions from YAML (atomic, robust) ---
	knowledgeActionYmlDir := "astra/agents/configs/actions/knowledge"
	learningActionsYAML, err := configs.LoadActionsYAMLInDir(knowledgeActionYmlDir)
	if err != nil {
		panic("Failed to load learning actions YAML configs: " + err.Error())
	}
	// Registration data pairing key, params, and function
	var longTermKnowledgeRegistrations = []struct {
		key    string
		params interface{}
		fn     interface{}
	}{
		{"create_long_term_knowledge", CreateLongTermKnowledgeParams{}, a.CreateLongTermKnowledgeAction},
		{"fetch_knowledge_types", struct{}{}, a.GetAllKnowledgeTypesForUser},
		{"get_all_long_term_knowledge_for_user", struct{}{}, a.GetAllLongTermKnowledgeForUserAction},
		{"get_all_long_term_knowledge_for_user_by_type", GetAllLongTermKnowledgeByTypeParams{}, a.GetAllLongTermKnowledgeForUserByTypeAction},
	}
	for _, reg := range longTermKnowledgeRegistrations {
		yamlCfg, ok := learningActionsYAML[reg.key]
		if !ok {
			panic("Missing YAML: " + reg.key)
		}
		a.register(ActionSpec{
			Name:        yamlCfg.Name,
			Description: yamlCfg.Description,
			Details:     yamlCfg.Details,
			Params:      reg.params,
			Fn:          reg.fn,
		})
	}
	// --- End YAML-driven learning actions registration ---

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
