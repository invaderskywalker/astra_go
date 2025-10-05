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

func (a *DataActions) AskFollowUpQuestions(params AskFollowUpQuestionsParams) (AskFollowUpQuestionsParams, error) {
	return params, nil
}
