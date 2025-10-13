package actions

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const testFile = "test_target.go"

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func restoreTestTarget() {
	cmds := [][]string{
		{"go", "fmt", "./..."},
		{"go", "vet", "./..."},
		{"go", "build", "./..."},
	}

	for _, cmdArgs := range cmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = "." // project root
		out, err := cmd.CombinedOutput()
		if len(out) > 0 {
			fmt.Printf("🔍 %s output:\n%s\n", strings.Join(cmdArgs, " "), string(out))
		}
		if err != nil {
			fmt.Printf("❌ %s failed: %v\n", strings.Join(cmdArgs, " "), err)
			return
		}
	}

	fmt.Println("✅ Go code formatted, vetted, and compiled successfully.")
}

// --- Test 1: Replace Line ---
func TestApplyCodeEdits_ReplaceKnownFile(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":           "replace",
				"target":         "userPrompt = fmt.Sprintf(",
				"context_before": "var userPrompt string",
				"replacement":    "userPrompt = fmt.Sprintf(`\n\tCurrent Date: %s\n\tPlease analyze and create a good thoughtful \n\texecution plan and output a single object\n\tPlease stick to the json output format and include all output in the JSON\n\n\t****important*****\n\t- Respond ONLY with valid JSON only stick to this format: %s\n\t- Any text outside the JSON is considered an error.\n\t`, time.Now().Format(\"2006-01-02\"), a.Config.OutputFormats.ExecutionStepOutputJSON)",
				"start":          "    userPrompt = fmt.Sprintf(`",
				"end":            ")",
				"position":       "replace",
				"file":           testFile,
			},
		},
	}

	result, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("replace failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "Current Date:") {
		t.Errorf("replacement not applied correctly; expected replacement snippet not found")
	}

	fmt.Println("✅ Replace test result:", result)
}

// --- Test 2: Insert After ---
func TestApplyCodeEdits_InsertAfterKnownFile(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":           "replace",
				"target":         "type FetchLearningKnowledgeParams struct {",
				"context_before": "// Params for creating a LearningKnowledge record",
				"position":       "after",
				"replacement":    "type FetchLearningKnowledgeParams struct {\n\tID     string                 `json:\"id,omitempty\"` // uuid string, optional\n\tUserID int                    `json:\"user_id,omitempty\"`\n\tLimit  int                    `json:\"limit,omitempty\"`\n\tFilters map[string]interface{} `json:\"filters,omitempty\"` // Additional filter fields; explicit fields take precedence\n}",
				"file":           "./learning_knowledge.go",
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("insert after failed: %v", err)
	}

	// out := readFile(testFile)
	// fmt.Println("out -- ", out)
	// if !strings.Contains(out, "INSERTED AFTER START") {
	// 	t.Errorf("insert after line missing in file")
	// }
}

// --- Test 3: Insert Before ---
func TestApplyCodeEdits_InsertBeforeKnownFile(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":     "insert",
				"target":   "fmt.Println(\"end\")",
				"position": "before",
				"content":  "    fmt.Println(\"INSERTED BEFORE END\")",
				"file":     testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("insert before failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "INSERTED BEFORE END") {
		t.Errorf("insert before line missing in file")
	}
}

// --- Test 4: Replace Block ---
func TestApplyCodeEdits_BlockReplaceKnownFile(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":        "replace",
				"start":       "fmt.Println(\"start\")",
				"end":         "fmt.Println(\"end\")",
				"replacement": "    fmt.Println(\"BLOCK REPLACED BETWEEN START AND END\")",
				"file":        testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("block replace failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "BLOCK REPLACED") {
		t.Errorf("block replace missing in file")
	}
}

// --- Test 5: Add Field to Struct ---
func TestApplyCodeEdits_AddStructField(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":     "insert",
				"target":   "FullName *string `json:\"full_name,omitempty\" gorm:\"type:varchar(255)\"`",
				"position": "after",
				"content":  "    PasswordHash string `json:\"password_hash\" gorm:\"type:varchar(255);not null\"`",
				"file":     testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("add struct field failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "PasswordHash") {
		t.Errorf("expected PasswordHash not added to struct")
	}
}

// --- Test 6: Add New DAO Method ---
func TestApplyCodeEdits_AddNewMethod(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":     "insert",
				"target":   "func (dao *UserDAO) GetUserByID(ctx context.Context, id int) (*models.User, error) {",
				"position": "before",
				"content": `
func (dao *UserDAO) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := dao.DB.WithContext(ctx).Model(&models.User{}).Count(&count).Error
	return count, err
}`,
				"file": testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("add method failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "CountUsers") {
		t.Errorf("expected CountUsers method not found")
	}
}

// --- Test 7: Create & Delete File ---
func TestApplyCodeEdits_CreateAndDeleteFile(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)
	tempFile := "test_created.go"

	// create new file
	paramsCreate := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":    "create_file",
				"file":    tempFile,
				"content": "package actions\n\nfunc CreatedFunc() {}",
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", paramsCreate)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}
	if _, err := os.Stat(tempFile); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// delete file
	paramsDelete := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type": "delete_file",
				"file": tempFile,
			},
		},
	}
	_, err = a.ExecuteAction("apply_code_edits", paramsDelete)
	if err != nil {
		t.Fatalf("delete file failed: %v", err)
	}
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("file was not deleted")
	}
}

func TestApplyCodeEdits_InsertWithContextBefore(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":           "insert",
				"target":         "fmt.Println(\"middle\")",
				"context_before": "fmt.Println(\"start\")",
				"position":       "after",
				"content":        "    fmt.Println(\"INSERTED AFTER CONTEXT BEFORE\")",
				"file":           testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("context insert failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "INSERTED AFTER CONTEXT BEFORE") {
		t.Errorf("context-based insert not found")
	}
}

func TestApplyCodeEdits_InsertWithContextAfter(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":          "insert",
				"target":        "fmt.Println(\"middle\")",
				"context_after": "fmt.Println(\"end\")",
				"position":      "before",
				"content":       "    fmt.Println(\"INSERTED USING CONTEXT AFTER\")",
				"file":          testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("context after insert failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "INSERTED USING CONTEXT AFTER") {
		t.Errorf("context after-based insert not found")
	}
}

func TestApplyCodeEdits_InsertAtEOF(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":    "insert",
				"target":  "__EOF__",
				"content": "// APPENDED AT END OF FILE",
				"file":    testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("insert EOF failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "// APPENDED AT END OF FILE") {
		t.Errorf("expected content not appended at EOF")
	}
}

func TestApplyCodeEdits_InsertAtBOF(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":    "insert",
				"target":  "__BOF__",
				"content": "// PREPENDED AT START OF FILE",
				"file":    testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("insert BOF failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.HasPrefix(out, "// PREPENDED AT START OF FILE") {
		t.Errorf("expected content not prepended at BOF")
	}
}

func TestApplyCodeEdits_IndentedBlockReplace(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":        "replace",
				"start":       "fmt.Println(\"start\")",
				"end":         "fmt.Println(\"end\")",
				"replacement": "    fmt.Println(\"BLOCK REPLACED WITH INDENTATION\")",
				"file":        testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("indented block replace failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "BLOCK REPLACED WITH INDENTATION") {
		t.Errorf("block replacement failed")
	}
}

func TestApplyCodeEdits_MultipleInOneBatch(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":        "replace",
				"target":      "fmt.Println(\"middle\")",
				"replacement": "fmt.Println(\"MULTI-REPLACED\")",
				"file":        testFile,
			},
			{
				"type":     "insert",
				"target":   "fmt.Println(\"end\")",
				"position": "before",
				"content":  "    fmt.Println(\"MULTI-INSERTED\")",
				"file":     testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("multi-edit failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "MULTI-INSERTED") || !strings.Contains(out, "MULTI-REPLACED") {
		t.Errorf("multiple edits not applied correctly")
	}
}

func TestApplyCodeEdits_TargetNotFound(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":     "insert",
				"target":   "fmt.Println(\"does not exist\")",
				"position": "after",
				"content":  "fmt.Println(\"SHOULD NOT PANIC\")",
				"file":     testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("target not found insert failed: %v", err)
	}

	out := readFile(testFile)
	if strings.Contains(out, "SHOULD NOT PANIC") {
		t.Errorf("unexpected insert on non-existent target")
	}
}

func TestApplyCodeEdits_InsertInsideFunction_GetUserByID(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	// Insert a debug log inside GetUserByID after the declaration
	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":        "replace",
				"file":        "../core/base_agent.go",
				"start":       "func (a *BaseAgent) generateNextExecutionPlan(roughPlan map[string]interface{}, stepIndex int, results any) (plan map[string]interface{}) {",
				"end":         "return plan\n}",
				"replacement": "func (a *BaseAgent) generateNextExecutionPlan(roughPlan map[string]interface{}, stepIndex int, results any) (plan map[string]interface{}) {\n\t// Default error return if something goes wrong\n\tdefer func() {\n\t\tif r := recover(); r != nil {\n\t\t\tlogging.ErrorLogger.Error(\"generateNextExecutionPlan failure\", zap.Any(\"recover\", r))\n\t\t\tplan = map[string]interface{}{\"error\": fmt.Sprint(r)}\n\t\t}\n\t}()\n\n\t// Get full action specs (params, returns, examples) from runtime registry\n\tfullActions := a.dataActions.ListActions()\n\n\tvar systemPrompt string\n\tvar userPrompt string\n\n\tsystemPrompt = fmt.Sprintf(`\n\t\tYou are Astra’s  sequential execution Planner.\n\n\t\tContext:\n\t\t- Full mind map plan: %s\n\t\t- Previous execution results: %s\n\t\t**Available Actions (full description with usage instruction):** \n\t\t<available_actions_with_full_description>\n\t\t%s\n\t\t</available_actions_with_full_description>\n\n\t\t## Decision Process\n\t\t**Description:**\n\t\t%s\n\n\t\tTask:\n\t\tYou are provided with a full mind map of responding \n\t\tto user query.\n\t\tAnd you are provided with all actions that you can take and \n\t\tall previous execution determined by you and their results.\n\n\t\tThink properly and present only the next single \n\t\tconcrete execution plan (single JSON object).\n\n\t\tRules:\n\t\t- Output exactly one JSON object and nothing else.\n\t\t- If no concrete action is required, set \"action\" to an empty string and return the schema.\n\n\t\t## Output Schema (stick to this)\n\t\t%s\n\t\t`,\n\t\tjsonutils.ToJSON(roughPlan),\n\t\tjsonutils.ToJSON(results),\n\t\tjsonutils.ToJSON(fullActions),\n\t\ta.Config.DecisionProcess.Description,\n\t\ta.Config.OutputFormats.ExecutionStepOutputJSON,\n\t)\n\n\tcurrentDateStr := time.Now().Format(\"January 2, 2006\")\n\tdatePreamble := fmt.Sprintf(\"Today's date is: %s.\\n\\n\", currentDateStr)\n\n\tuserPrompt = datePreamble + fmt.Sprintf(`\n\t\tPlease analyze and create a good thoughtful \n\t\texecution plan and output a single object\n\t\tPlease stick to the json output format and include all output in the JSON\n\n\t\t****important*****\n\t\t- Respond ONLY with valid JSON only stick to this format: %s\n\t\t- Any text outside the JSON is considered an error.\n\t\t- Dont keep repeating any action - be sensible, you are not some small time rookie, you are supposed to my JARVIS\n\t\t`,\n\t\ta.Config.OutputFormats.ExecutionStepOutputJSON,\n\t)\n\n\treq := llm.ChatRequest{\n\t\tModel: DefaultModel,\n\t\tMessages: []llm.Message{\n\t\t\t{Role: \"system\", Content: systemPrompt},\n\t\t\t{Role: \"user\", Content: userPrompt},\n\t\t},\n\t\tStream: false,\n\t}\n\n\tresp, err := a.LLM.Run(context.Background(), req)\n\tif err != nil {\n\t\tpanic(fmt.Errorf(\"failed to create plan: %w\", err))\n\t}\n\n\tfmt.Println(\"\\nexec plan created --- \", resp)\n\n\trespJSON := jsonutils.ExtractJSON(resp)\n\tif err := json.Unmarshal([]byte(respJSON), &plan); err != nil {\n\t\tpanic(fmt.Errorf(\"invalid plan format: %w\", err))\n\t}\n\n\ta.ExecutionPlans = append(a.ExecutionPlans, plan)\n\treturn plan\n}",
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("insert inside function failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "DEBUG: Entered GetUserByID") {
		t.Errorf("expected insert inside GetUserByID not found")
	}
	if strings.Contains(out, "DEBUG: Entered GetUserByID2") {
		t.Errorf("accidentally inserted inside GetUserByID2")
	}
}

func TestApplyCodeEdits_ReplaceInsideFunction_GetUserByID2(t *testing.T) {
	defer restoreTestTarget()
	a := setupTestEnv(t)

	// Replace an inner line in GetUserByID2
	params := map[string]interface{}{
		"edits": []map[string]interface{}{
			{
				"type":        "replace",
				"file":        "./actions.go",
				"start":       "// --- Register learning actions from YAML (atomic, robust) ---",
				"end":         "// --- End YAML-driven learning actions registration ---",
				"replacement": "// --- Register learning actions from YAML (loop-based/atomic) ---\nlearningYAMLDir := \"astra/agents/configs/actions/learning\"\nlearningActionsYAML, err := configs.LoadActionsYAMLInDir(learningYAMLDir)\nif err != nil {\n\tpanic(\"Failed to load learning actions YAML configs: \" + err.Error())\n}\n\n// Specs for all YAML-driven learning actions\ntype learningActionSpec struct {\n\tYAMLKey   string\n\tParams    interface{}\n\tHandlerFn interface{}\n}\nvar learningSpecs = []learningActionSpec{\n\t{\"create_long_term_knowledge\", CreateLearningKnowledgeParams{}, a.CreateLearningKnowledgeAction},\n\t{\"update_learning_knowledge\", UpdateLearningKnowledgeParams{}, a.UpdateLearningKnowledgeAction},\n\t{\"get_all_long_term_knowledge_for_user\", struct{}{}, a.GetAllLearningKnowledgeForUserAction},\n\t{\"get_all_long_term_knowledge_for_user_by_type\", GetAllLearningKnowledgeByTypeParams{}, a.GetAllLearningKnowledgeForUserByTypeAction},\n}\nfor _, spec := range learningSpecs {\n\tyamlCfg, ok := learningActionsYAML[spec.YAMLKey]\n\tif !ok {\n\t\tpanic(fmt.Sprintf(\"Missing YAML: %s\", spec.YAMLKey))\n\t}\n\tif spec.HandlerFn == nil {\n\t\tpanic(fmt.Sprintf(\"No handler function for: %s\", spec.YAMLKey))\n\t}\n\ta.register(ActionSpec{\n\t\tName:        yamlCfg.Name,\n\t\tDescription: yamlCfg.Description,\n\t\tDetails:     yamlCfg.Details,\n\t\tParams:      spec.Params,\n\t\tFn:          spec.HandlerFn,\n\t})\n}\n// --- End YAML-driven learning actions registration ---",
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("replace inside function failed: %v", err)
	}

	// out := readFile(testFile)
	// if !strings.Contains(out, "mock error from GetUserByID2") {
	// 	t.Errorf("replacement inside GetUserByID2 not applied correctly")
	// }
	// if strings.Contains(out, "mock error from GetUserByID\"") {
	// 	t.Errorf("replacement wrongly affected GetUserByID")
	// }
}
