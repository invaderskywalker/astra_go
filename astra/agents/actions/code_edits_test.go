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
				"type":     "insert",
				"target":   "fmt.Println(\"start\")",
				"position": "after",
				"content":  "    fmt.Println(\"INSERTED AFTER START\")",
				"file":     testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("insert after failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "INSERTED AFTER START") {
		t.Errorf("insert after line missing in file")
	}
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
				"type":           "insert",
				"target":         "err := dao.DB.WithContext(ctx).First(&user, id).Error",
				"context_before": "func (dao *UserDAO) GetUserByID(",
				"position":       "after",
				"content":        "    fmt.Println(\"DEBUG: Entered GetUserByID\")",
				"file":           testFile,
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
				"type":           "replace",
				"target":         "return &user, nil",
				"context_before": "func (dao *UserDAO) GetUserByID2(",
				"replacement":    "return &user, fmt.Errorf(\"mock error from GetUserByID2\")",
				"file":           testFile,
			},
		},
	}

	_, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("replace inside function failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "mock error from GetUserByID2") {
		t.Errorf("replacement inside GetUserByID2 not applied correctly")
	}
	if strings.Contains(out, "mock error from GetUserByID\"") {
		t.Errorf("replacement wrongly affected GetUserByID")
	}
}
