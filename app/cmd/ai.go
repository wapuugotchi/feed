package cmd

import "wapuugotchi/feed/app/ai"

const defaultPattern = "Text:\n\n%s"

// TransformTextByAi uses the default prompt for the CLI.
func TransformTextByAi(text string) (string, error) {
	return ai.TransformText(defaultPattern, text)
}
