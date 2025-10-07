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
				"target":         "fmt.Println(\"middle\")",
				"context_before": "func DemoFunction(",
				"replacement":    "fmt.Println(\"REPLACED MIDDLE\")",
				"file":           testFile,
			},
		},
	}

	result, err := a.ExecuteAction("apply_code_edits", params)
	if err != nil {
		t.Fatalf("replace failed: %v", err)
	}

	out := readFile(testFile)
	if !strings.Contains(out, "REPLACED MIDDLE") {
		t.Errorf("replacement not found in file")
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
	tempFile := "astra/agents/actions/test_created.go"

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
