package main

import (
	"fmt"
	"strings"
	"time"
)

var antLogoFacingLeft = []string{
	`/\/\`,
	`  \_\  _..._`,
	`  (" )(_..._)`,
	`   ^^  // \\`,
}

var antLogo = mirrorASCIIArt(antLogoFacingLeft)

const antmoveTagline = "Move apps. Keep them running."
const antHeaderGap = 1

func mirrorASCIIArt(lines []string) []string {
	width := 0
	for _, line := range lines {
		if len(line) > width {
			width = len(line)
		}
	}

	mirrored := make([]string, len(lines))
	for i, line := range lines {
		padded := []rune(line + strings.Repeat(" ", width-len(line)))
		for left, right := 0, len(padded)-1; left < right; left, right = left+1, right-1 {
			padded[left], padded[right] = mirrorASCIISymbol(padded[right]), mirrorASCIISymbol(padded[left])
		}
		if len(padded)%2 == 1 {
			middle := len(padded) / 2
			padded[middle] = mirrorASCIISymbol(padded[middle])
		}
		mirrored[i] = strings.TrimRight(string(padded), " ")
	}
	return mirrored
}

func mirrorASCIISymbol(symbol rune) rune {
	switch symbol {
	case '/':
		return '\\'
	case '\\':
		return '/'
	case '(':
		return ')'
	case ')':
		return '('
	case '[':
		return ']'
	case ']':
		return '['
	case '{':
		return '}'
	case '}':
		return '{'
	case '<':
		return '>'
	case '>':
		return '<'
	default:
		return symbol
	}
}

func brandHeaderLine(index int) string {
	return brandHeaderLineFor(antLogo[:], index)
}

func brandHeaderLineFor(logo []string, index int) string {
	logoWidth := 0
	for _, line := range logo {
		if len(line) > logoWidth {
			logoWidth = len(line)
		}
	}
	line := logo[index]
	var right string
	switch index {
	case len(logo) - 2:
		right = "antmove"
	case len(logo) - 1:
		right = antmoveTagline
	default:
		return line
	}
	return fmt.Sprintf("%-*s    %s", logoWidth, line, right)
}

func styledBrandHeaderLine(index int) string {
	return styledBrandHeaderLineFor(antLogo[:], index)
}

func styledBrandHeaderLineFor(logo []string, index int) string {
	line := logo[index]
	var right string
	var rightStyle = tuiBrandStyle
	switch index {
	case len(logo) - 2:
		right = "antmove"
	case len(logo) - 1:
		right = antmoveTagline
		rightStyle = tuiTaglineStyle
	default:
		return tuiBrandStyle.Render(line)
	}
	logoWidth := 0
	for _, logoLine := range logo {
		if len(logoLine) > logoWidth {
			logoWidth = len(logoLine)
		}
	}
	padding := strings.Repeat(" ", logoWidth-len(line)+4)
	return tuiBrandStyle.Render(line) + padding + rightStyle.Render(right)
}

func animatedBrandHeaderLineFor(logo []string, index, offset, textStart int) string {
	line := strings.Repeat(" ", offset) + fmt.Sprintf("%-*s", antArtworkWidth(), logo[index])
	right := brandHeaderRightText(logo, index)
	if right == "" {
		return strings.TrimRight(line, " ")
	}
	padding := maxInt(antHeaderGap, textStart-offset-antArtworkWidth())
	return line + strings.Repeat(" ", padding) + right
}

func styledAnimatedBrandHeaderLineFor(logo []string, index, offset, textStart int) string {
	line := tuiBrandStyle.Render(strings.Repeat(" ", offset) + fmt.Sprintf("%-*s", antArtworkWidth(), logo[index]))
	right := brandHeaderRightText(logo, index)
	if right == "" {
		return strings.TrimRight(line, " ")
	}
	padding := maxInt(antHeaderGap, textStart-offset-antArtworkWidth())
	style := tuiBrandStyle
	if index == len(logo)-1 {
		style = tuiTaglineStyle
	}
	return line + strings.Repeat(" ", padding) + style.Render(right)
}

func brandHeaderRightText(logo []string, index int) string {
	switch index {
	case len(logo) - 2:
		return "antmove"
	case len(logo) - 1:
		return antmoveTagline
	default:
		return ""
	}
}

func logoWidth(logo []string) int {
	width := 0
	for _, line := range logo {
		width = maxInt(width, len(line))
	}
	return width
}

func antHeaderTextStart(width int) int {
	return antArtworkWidth() + antMaxOffset(width) + antHeaderGap
}

func antMaxOffset(width int) int {
	if width < 20 {
		width = 80
	}
	available := width - antArtworkWidth() - len(antmoveTagline) - antHeaderGap
	return minInt(maxAntTravelOffset, maxInt(0, available))
}

func antHeaderWidth(width int) int {
	return minInt(width, antHeaderTextStart(width)+len(antmoveTagline))
}

func antArtworkWidth() int {
	return maxInt(logoWidth(antLogoFacingLeft), logoWidth(antLogo))
}

func formatBytes(value int64) string {
	if value < 0 {
		value = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	size := float64(value)
	unit := 0
	for size >= 1000 && unit < len(units)-1 {
		size /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d B", value)
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}

func approximateDuration(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	seconds := int(value.Round(time.Second).Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	seconds %= 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}
	hours := minutes / 60
	minutes %= 60
	return fmt.Sprintf("%dh %02dm", hours, minutes)
}

func truncate(value string, max int) string {
	runes := []rune(value)
	if max < 1 || len(runes) <= max {
		return value
	}
	if max == 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}
