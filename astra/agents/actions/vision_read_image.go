package actions

type ReadImageWithVisionParams struct {
	ImagePaths      []string `json:"image_paths"`
	UserInstruction string   `json:"user_instruction,omitempty"`
}

type VisionImageResult struct {
	ImagePath         string `json:"image_path"`
	VisionUsed        bool   `json:"vision_used"`
	VisionDescription string `json:"vision_description,omitempty"`
	Error             string `json:"error,omitempty"`
}
