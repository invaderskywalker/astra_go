// astra/utils/color.go
package color

import (
	"github.com/fatih/color"
)

var (
	promptColor       = color.New(color.FgCyan, color.Bold)
	infoColor         = color.New(color.FgGreen)
	warningColor      = color.New(color.FgYellow, color.Bold)
	errorColor        = color.New(color.FgRed, color.Bold)
	agentRespColor    = color.New(color.FgHiYellow, color.Bold)
	finalSuccessColor = color.New(color.FgGreen, color.Bold)
	finalFailColor    = color.New(color.FgMagenta, color.Bold)
)

func ColorPrompt(s string) string {
	return promptColor.Sprint(s)
}

func ColorInfo(s string) string {
	return infoColor.Sprint(s)
}

func ColorWarning(s string) string {
	return warningColor.Sprint(s)
}

func ColorError(s string) string {
	return errorColor.Sprint(s)
}

func ColorAgentResponse(s string) string {
	return agentRespColor.Sprint(s)
}

func ColorFinalSuccess(s string) string {
	return finalSuccessColor.Sprint(s)
}

func ColorFinalFail(s string) string {
	return finalFailColor.Sprint(s)
}

// func DisableColorIfNotTTY() {
// 	if !color.Output.(*os.File).IsTerminal() {
// 		color.NoColor = true
// 	}
// }
