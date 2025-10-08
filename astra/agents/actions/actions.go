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
	Details     string      `json:"details"`
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
		Details: `
			# 🧠 Astra Code Editing Engine

			The Astra Code Editing Engine enables intelligent, context-aware code modifications 
			inside the Astra Agent framework. 
			It supports operations such as inserting, replacing, creating, and deleting files 

			---

			## ⚙️ Core Files

					• Implementation: astra/astra/agents/actions/code_edits.go  
					• Tests: astra/astra/agents/actions/code_edits_test.go  

			---

			## 🧩 Core Data Structures

					• CodeEdit – represents a single modification:
						- type: "insert" | "replace" | "create_file" | "delete_file"
						- file: path to target file
						- target: reference line to locate edit
						- start / end: block boundaries (for multi-line replaces)
						- replacement: content to replace existing code
						- content: new content for inserts or new files
						- position: "before" | "after" (default "after")
						- context_before / context_after: lines near target for safe matching
						- Special targets: "__BOF__" (file start), "__EOF__" (file end)

					• ApplyCodeEditsParams – batch container:
						- Edits []CodeEdit

					• ApplyCodeEditsResult – operation output:
						- Success: true/false
						- EditsApplied: number of edits
						- Error: error message if failed

			---

			## 🧠 Execution Flow

					1. Validate Edits
							Each edit is checked for valid file paths and grouped by file.

					2. Read & Sanity Check
							Loads file lines, ensures “package” exists for Go files.

					3. Apply Edits
							Each edit type is processed individually:
								- create_file → writes a new file
								- delete_file → removes file if exists
								- insert → adds new content before/after a target
								- replace → swaps a target line or block

					4. Context Matching
							The engine finds the correct line using target + context hints.
							It searches nearby lines (window ≈ 25) to avoid false matches.

			---

			## 🧪 Example Edit Instructions

					✅ Replace a Line
					{
						"edits": [
							{
								"type": "replace",
								"target": "fmt.Println(\"middle\")",
								"context_before": "func DemoFunction(",
								"replacement": "fmt.Println(\"REPLACED MIDDLE\")",
								"file": "test_target.go"
							}
						]
					}

					✅ Insert Inside a Function
					{
						"edits": [
							{
								"type": "insert",
								"target": "err := dao.DB.WithContext(ctx).First(&user, id).Error",
								"context_before": "func (dao *UserDAO) GetUserByID(",
								"position": "after",
								"content": "    fmt.Println(\"DEBUG: Entered GetUserByID\")",
								"file": "test_target.go"
							}
						]
					}

					✅ Replace Return in Specific Function
					{
						"edits": [
							{
								"type": "replace",
								"target": "return &user, nil",
								"context_before": "func (dao *UserDAO) GetUserByID2(",
								"replacement": "return &user, fmt.Errorf(\"mock error from GetUserByID2\")",
								"file": "test_target.go"
							}
						]
					}

					✅ Insert at End of File
					{
						"edits": [
							{
								"type": "insert",
								"target": "__EOF__",
								"content": "// APPENDED AT END OF FILE",
								"file": "test_target.go"
							}
						]
					}

			---


			**In essence:**  
					The Astra Code Editing Engine gives your AI agents the power to intelligently 
					edit source code — line-by-line, block-by-block, or function-by-function — 
					while preserving structure, formatting, and intent.

		`,
		Params: ApplyCodeEditsParams{},
		Examples: []string{
			``,
		},
		Fn: a.applyCodeEdits,
	})

	a.register(ActionSpec{
		Name:        "fetch_file_structure_in_this_repo",
		Description: "Fetches the file and folder structure of the current repository using the system `tree` command.",
		Params:      FetchFileStructureParams{},
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
		Examples: []string{
			``,
		},
		Fn: a.ScrapeURLs,
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
		Examples: []string{
			``,
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
