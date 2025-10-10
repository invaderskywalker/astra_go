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
	Params      interface{} `json:"params"` // Struct type for parameters
	Fn          interface{} `json:"-"`      // Actual function (not serialized)
}

// NewDataActions initializes the DataActions registry.
func NewDataActions(db *gorm.DB) *DataActions {

	a := &DataActions{
		actions: make(map[string]ActionSpec),
		db:      db,
	}

	a.register(ActionSpec{
		Name: "apply_code_edits",
		Description: `
			Applies a list of code edits (replace, insert, create_file, delete_file) to source files. 
			Only to be triggered when code edit is required. 
			Always see this file for examples on how to add: code_edits_test
		`,
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

			## 🧠 LLM Prompt Guidelines for Safe Code Editing

				When generating code edit instructions for this action, **follow these mandatory rules**:

				1. **Preserve formatting and indentation exactly.**
				- Do not add or remove backslashes inside string literals.
				- Do not escape existing ` + "`\\n` or `\\t`" + ` sequences.
				- Do not collapse multi-line raw strings or alter backticks.

				2. **Respect raw string literals.**
				- If you see a back-tick raw string,  
					keep all internal newlines and tabs exactly as written.
				- Never replace backticks with double quotes.

				3. **No concatenation unless explicitly required.**
				- Avoid breaking raw strings into "+" pieces.
				- Keep long literals intact unless splitting is unavoidable for variable interpolation.

				4. **Edit only the target block.**
				- Never reindent or reformat the entire file.
				- Only modify the specified region around the <target> and <context_before>.

				5. **Preserve valid Go syntax.**
				- Ensure all edits compile.
				- Do not introduce stray slashes, quotes, or broken multiline strings.

				6. **For simple replacements (single-line changes), prefer direct substitution.**
				- When both <target> and <replacement> are provided without <start>/<end>, 
					Astra should replace the single line containing <target> with <replacement>.
				- Avoid deleting or moving code unless explicitly instructed.
				- This ensures minimal disruption and avoids structural errors.


				---


			**In essence:**  
					The Astra Code Editing Engine gives your AI agents the power to intelligently 
					edit source code — line-by-line, block-by-block, or function-by-function — 
					while preserving structure, formatting, and intent.

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
					"context": "",
					"goal": ""
				}
			}
		`,
		Params: struct{}{}, // no params required, but agent may internally provide context
		Fn:     nil,        // intentionally nil — handled internally in BaseAgent
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
