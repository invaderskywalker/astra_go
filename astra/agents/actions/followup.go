package actions

type AskFollowUpQuestionsParams struct {
	Questions []string `json:"questions"`
}

type AskFollowUpQuestionsResult struct {
	FollowUps []FollowUpItem `json:"follow_ups"`
}

// FollowUpItem represents one follow-up question and its placeholder answer.
type FollowUpItem struct {
	Question string `json:"question"`
}

func (a *DataActions) AskFollowUpQuestions(params AskFollowUpQuestionsParams) ActionResult {
	followUps := make([]FollowUpItem, 0, len(params.Questions))
	for _, question := range params.Questions {
		if question != "" {
			followUps = append(followUps, FollowUpItem{Question: question})
		}
	}
	return ActionResult{Success: true, Summary: "Follow-up questions required", Diagnostics: AskFollowUpQuestionsResult{FollowUps: followUps}}
}
