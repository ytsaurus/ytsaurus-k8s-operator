package canonize

import "strings"

type Color string

const (
	colorRed   Color = "\033[31m"
	colorGreen Color = "\033[32m"
	colorCyan  Color = "\033[36m"
)

const colorReset = "\033[0m"

func addColorsToDiff(text string) string {
	coloredText := ""
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(l, "+") {
			l = string(colorGreen) + l + colorReset
		} else if strings.HasPrefix(l, "-") {
			l = string(colorRed) + l + colorReset
		} else if strings.HasPrefix(l, "@@") {
			l = string(colorCyan) + l + colorReset
		}
		coloredText += l + "\n"
	}
	return coloredText
}
