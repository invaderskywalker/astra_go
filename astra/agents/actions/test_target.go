package actions

import (
	"astra/astra/sources/psql/models"
	"astra/astra/utils/jsonutils"
	"astra/astra/utils/logging"
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// test_target.go — intentionally simple and editable file for code edit testing

func DemoFunction() {
	fmt.Println("start")
	fmt.Println("middle")
	fmt.Println("end")
}

type DemoStruct struct {
	Name string
}

func UnusedFunction() {
	fmt.Println("this should be replaced")
}

type User struct {
	ID       int     `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string  `json:"username" gorm:"type:varchar(255);not null"`
	Email    string  `json:"email" gorm:"type:varchar(255);not null"`
	FullName *string `json:"full_name,omitempty" gorm:"type:varchar(255)"`
}

type UserDAO struct {
	DB *gorm.DB
}

func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{DB: db}
}

func (dao *UserDAO) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var user models.User
	err := dao.DB.WithContext(ctx).First(&user, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func generateNextExecutionPlan(roughPlan map[string]interface{}, stepIndex int, results any) (plan map[string]interface{}) {
	// Default error return if something goes wrong
	defer func() {
		if r := recover(); r != nil {
			logging.ErrorLogger.Error("generateNextExecutionPlan failure", zap.Any("recover", r))
			plan = map[string]interface{}{"error": fmt.Sprint(r)}
		}
	}()

	// Get full action specs (params, returns, examples) from runtime registry
	fullActions := ""
	actionsJSON, _ := json.MarshalIndent(fullActions, "", "  ")
	actionsJSONStr := string(actionsJSON)

	var systemPrompt string
	var userPrompt string

	systemPrompt = fmt.Sprintf(`
		You are Astra’s  sequential execution Planner.

		Context:
		- Full mind map plan: %s
		- Previous execution results: %s
		- Available actions (full spec): %s

		Task:
		You are provided with a full mind map of responding 
		to user query.
		And you are provided with all actions that you can take and 
		all previous execution determined by you and their results.

		Think properly and present only the next single 
		concrete execution plan (single JSON object).

		

		Rules:
		- Output exactly one JSON object and nothing else.
		- If no concrete action is required, set "action" to an empty string and return the schema.

		## Output Schema (stick to this)
			%s
		`,
		jsonutils.ToJSON(roughPlan),
		jsonutils.ToJSON(results),
		actionsJSONStr,
		"",
	)

	// fmt.Println("debug generateNextExecutionPlan prompt ", systemPrompt)

	userPrompt = fmt.Sprintf(`
		Please analyze and create a good thoughtful 
		execution plan and output a single object
		Please stick to the json output format and include all output in the JSON

		****important*****
		- Respond ONLY with valid JSON only stick to this format: %s
		- Any text outside the JSON is considered an error.
		`,
		"a.Config.OutputFormats.ExecutionStepOutputJSON",
	)
	fmt.Println("systyem --- ", systemPrompt, userPrompt)

	// req := llm.ChatRequest{
	// 	Model: DefaultModel,
	// 	Messages: []llm.Message{
	// 		{Role: "system", Content: systemPrompt},
	// 		{Role: "user", Content: userPrompt},
	// 	},
	// 	Stream: false,
	// }

	// resp, err := a.LLM.Run(context.Background(), req)
	// if err != nil {
	// 	panic(fmt.Errorf("failed to create plan: %w", err))
	// }
	resp := ""

	fmt.Println("\nexec plan created --- ", resp)

	respJSON := jsonutils.ExtractJSON(resp)
	respJSON = jsonutils.CleanJSON(respJSON)
	if err := json.Unmarshal([]byte(respJSON), &plan); err != nil {
		panic(fmt.Errorf("invalid plan format: %w", err))
	}

	// persist for traceability
	// a.storeState("execution_step_expand", plan)
	// a.ExecutionPlans = append(a.ExecutionPlans, plan)
	return plan
}
