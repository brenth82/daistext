package daisy

import (
	"regexp"
	"strings"
)

var (
	reporterEmail  = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	reporterHandle = regexp.MustCompile(`(?:^|\s)@[a-zA-Z0-9_]+\b`)
	reporterName   = regexp.MustCompile(`^[\p{Lu}][\p{L}.'’\-]+(?: [\p{Lu}][\p{L}.'’\-]+){1,3}$`)
	reporterBio    = regexp.MustCompile(`^[\p{Lu}][\p{L}.'’\-]+(?: [\p{Lu}][\p{L}.'’\-]+){1,3} (?i:is (?:an? |the )(?:[\p{L}\-]+ ){0,5}(?:reporter|columnist|journalist|correspondent) (?:for|at|with) )`)
)

// SectionBlocks supplies the same text to preview and export.
func (b *Book) SectionBlocks(index int, skipReporterDetails bool) []Block {
	section := b.Sections[index]
	if !skipReporterDetails || !b.Newspaper || !section.article {
		return section.Blocks
	}
	return withoutReporterFooter(section.Blocks)
}

func withoutReporterFooter(blocks []Block) []Block {
	end := len(blocks)
	if end > 0 && blocks[end-1].Level == 0 {
		// NEWSLINE may append a short category marker after the reporter footer.
		label, found := strings.CutSuffix(blocks[end-1].Text, " .")
		if found && len(label) <= 30 {
			end--
		}
	}

	start := end
	for start > 0 && end-start < 4 && blocks[start-1].Level == 0 && isReporterFooter(blocks[start-1].Text) {
		start--
	}
	// Some articles use a separate name and role, optionally followed by
	// an uppercase publication affiliation.
	roleEnd := start
	if roleEnd > 0 {
		text := blocks[roleEnd-1].Text
		if blocks[roleEnd-1].Level == 0 && len(text) <= 70 && text == strings.ToUpper(text) &&
			(strings.Contains(text, " NETWORK") || strings.Contains(text, " NEWSROOM")) {
			roleEnd--
		}
	}
	if roleEnd >= 2 && blocks[roleEnd-1].Level == 0 && blocks[roleEnd-2].Level == 0 {
		role := strings.ToLower(strings.TrimSuffix(blocks[roleEnd-1].Text, "."))
		switch role {
		case "guest columnist", "staff reporter", "staff writer", "columnist", "correspondent":
			if reporterName.MatchString(blocks[roleEnd-2].Text) {
				start = roleEnd - 2
			}
		}
	}
	if start == end {
		return blocks
	}
	result := append([]Block(nil), blocks[:start]...)
	return append(result, blocks[end:]...)
}

func isReporterFooter(text string) bool {
	if len(text) > 500 {
		return false
	}
	if reporterBio.MatchString(text) {
		return true
	}
	lower := strings.ToLower(text)
	if reporterEmail.MatchString(text) &&
		(strings.HasPrefix(lower, "contact ") || strings.HasPrefix(lower, "reach ") ||
			strings.HasPrefix(lower, "email ") || strings.Contains(lower, " can be reached at ")) {
		return true
	}
	return strings.HasPrefix(lower, "follow ") && reporterHandle.MatchString(text)
}
