// Package speech prepares DAISY prose for English text-to-speech readers.
package speech

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

func SupportsLanguage(tag string) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" || strings.EqualFold(tag, "English") {
		return true
	}
	parsed, err := language.Parse(tag)
	base, confidence := parsed.Base()
	return err == nil && confidence == language.Exact && base.String() == "en"
}

var punctuation = strings.NewReplacer(
	"‘", "'", "’", "'", "“", `"`, "”", `"`, "…", "...",
	"·", " ", "•", " ", "\u00ad", "", "−", "-", "⁄", "/",
)

var (
	mixedFraction = regexp.MustCompile(`([0-9])([¼½¾⅐⅑⅒⅓⅔⅕⅖⅗⅘⅙⅚⅛⅜⅝⅞])`)
	urlOrEmail    = regexp.MustCompile(`(?i)https?://[^\s<>,;"']+|[a-z0-9._%+'-]+@[a-z0-9-]+(?:\.[a-z0-9-]+)*\.[a-z]{2,}`)
	social        = regexp.MustCompile(`[@#][\pL\pN_]+`)
	emailPieces   = regexp.MustCompile(`[A-Za-z]+|[0-9]+|[._%+'@-]`)
	dashes        = regexp.MustCompile(`[\p{Zs}\t]*[–—][\p{Zs}\t]*`)
	whitespace    = regexp.MustCompile(`[\p{Zs}\t]+`)
	beforePunct   = regexp.MustCompile(` +([,.;:!?])`)
	meridiemPunct = regexp.MustCompile(`\b(AM|PM)\.([,;:!?]| +\p{Ll})`)
	repeated      = regexp.MustCompile(`!{2,}|\?{2,}|\.{4,}|,{2,}`)
)

// DAISY import already handles markup and entities. Adapt the text rules from
// Transcodio's document preparation and pronunciation polish to plain blocks.
func Normalize(text string) string {
	text = mixedFraction.ReplaceAllString(text, "${1} and ${2}")
	text = punctuation.Replace(norm.NFKC.String(text))
	text = whitespace.ReplaceAllString(text, " ")
	text = urlOrEmail.ReplaceAllStringFunc(text, speakAddress)
	text = replaceTokens(text, social, func(token string) string {
		if token[0] == '@' {
			return "at " + token[1:]
		}
		return token[1:]
	})
	text = replaceTokens(text, abbreviationPattern, expandAbbreviation)
	text = replaceTokens(text, romanPattern, speakRoman)
	text = speakNumericTokens(text)
	text = dashes.ReplaceAllString(text, ", ")
	text = replaceTokens(text, caps, speakCaps)
	text = rewriteHomographs(text)
	text = whitespace.ReplaceAllString(text, " ")
	text = beforePunct.ReplaceAllString(text, "$1")
	text = meridiemPunct.ReplaceAllString(text, "$1$2")
	text = repeated.ReplaceAllStringFunc(text, func(token string) string {
		if token[0] == '.' {
			return "..."
		}
		return token[:1]
	})
	return strings.TrimSpace(text)
}

func speakAddress(token string) string {
	if strings.Contains(token, "://") {
		// Preserve sentence punctuation consumed by the URL pattern.
		link := strings.TrimRight(token, ".!?:)]}–—")
		return "link" + token[len(link):]
	}
	parts := emailPieces.FindAllString(token, -1)
	for i, part := range parts {
		switch part {
		case ".":
			parts[i] = "dot"
		case "_":
			parts[i] = "underscore"
		case "-":
			parts[i] = "dash"
		case "+":
			parts[i] = "plus"
		case "%":
			parts[i] = "percent"
		case "'":
			parts[i] = "apostrophe"
		case "@":
			parts[i] = "at"
		default:
			if part[0] >= '0' && part[0] <= '9' {
				parts[i] = speakDigits(part, " ")
			}
		}
	}
	return strings.Join(parts, " ")
}

var abbreviations = map[string]string{
	"mr.": "Mister", "mrs.": "Missus", "ms.": "Miss", "dr.": "Doctor",
	"prof.": "Professor", "sgt.": "Sergeant", "cpl.": "Corporal",
	"lt.": "Lieutenant", "col.": "Colonel", "gen.": "General", "st.": "Saint",
	"ave.": "Avenue", "blvd.": "Boulevard", "dept.": "Department",
	"vs.": "versus", "vs": "versus", "etc.": "et cetera",
	"e.g.": "for example", "i.e.": "that is", "w/": "with", "w/o": "without",
	"approx.": "approximately", "appt.": "appointment", "info.": "information",
	"FYI": "for your information", "IMHO": "in my humble opinion", "IRL": "in real life",
	"AKA": "also known as", "ASAP": "as soon as possible", "TBD": "to be determined",
	"TBA": "to be announced", "RSVP": "please respond", "&": "and",
}

var abbreviationPattern = regexp.MustCompile(
	`(?i:mr\.|mrs\.|ms\.|dr\.|prof\.|sgt\.|cpl\.|lt\.|col\.|gen\.|st\.|ave\.|blvd\.|dept\.|vs\.?|etc\.|e\.g\.|i\.e\.|w/o|w/|approx\.|appt\.|info\.)|FYI|IMHO|IRL|AKA|ASAP|TBD|TBA|RSVP|&`)

func expandAbbreviation(token string) string {
	if expansion, ok := abbreviations[token]; ok {
		return expansion
	}
	return abbreviations[strings.ToLower(token)]
}

var romanPattern = regexp.MustCompile(
	`(?i:Chapter|Part|Section|Act|Volume|Book|Episode|Phase|Stage)[ \t]+[IVXL]+`)

func speakRoman(token string) string {
	firstSpace := strings.IndexAny(token, " \t")
	label, numeral := token[:firstSpace], strings.TrimSpace(token[firstSpace:])
	// Match the canonical numerals supported by Transcodio (I–XL and L).
	if numeral == "L" {
		return label + " fiftieth"
	}
	tens := []string{"", "X", "XX", "XXX", "XL"}
	ones := []string{"", "I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX"}
	for n := 1; n <= 40; n++ {
		if numeral == tens[n/10]+ones[n%10] {
			return label + " " + ordinal(n)
		}
	}
	return token
}

var caps = regexp.MustCompile(`[A-Z]{2,}`)

func speakCaps(word string) string {
	switch word {
	case "AM", "PM", "OK", "NASA", "NATO", "UNESCO", "UNICEF", "COVID", "DAISY", "AIDS", "FEMA", "OSHA", "HUD", "OPEC":
		return word
	case "AI", "BBC", "BRF", "CBS", "CIA", "CNN", "CPU", "DA", "DNA", "DVD", "EU", "FAQ", "FBI", "FCC", "FDA", "FDR",
		"FEC", "FM", "GB", "GPS", "GPU", "HIV", "HTML", "HTTP", "HTTPS", "IBM", "ID", "IRS", "IT", "JSON", "LLC",
		"ML", "NBA", "NBC", "NFB", "NFL", "NPR", "NYC", "OPF", "PC", "PDF", "PTSD", "RNA", "SEO", "SMS",
		"SSD", "TTS", "TV", "TXT", "UK", "UN", "URL", "US", "USA", "USB", "UTC", "VIP", "XML", "ZIP":
		return strings.Join(strings.Split(word, ""), " ")
	}
	if len(word) >= 4 {
		return word[:1] + strings.ToLower(word[1:])
	}
	return word
}

var homographs = []struct {
	pattern     *regexp.Regexp
	spelling    *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(?i)\b(?:(?:has|have|had|been)(?: +already)?) +read\b`), regexp.MustCompile(`(?i)\bread\b`), "red"},
	{regexp.MustCompile(`(?i)\btear +(?:down|apart|open|off|into|up (?:the|this|that))\b`), regexp.MustCompile(`(?i)\btear\b`), "tare"},
	{regexp.MustCompile(`(?i)\bwind +(?:the clock|the watch|up (?:the|this|that|at|with))\b`), regexp.MustCompile(`(?i)\bwind\b`), "wynd"},
	{regexp.MustCompile(`(?i)\bminute +(?:details?|differences?|particles?|amounts?|changes?|variations?)\b`), regexp.MustCompile(`(?i)\bminute\b`), "mynoot"},
	{regexp.MustCompile(`(?i)\bbass +(?:guitars?|lines?|players?|notes?|clefs?|voices?|drums?|trombones?)\b`), regexp.MustCompile(`(?i)\bbass\b`), "base"},
	{regexp.MustCompile(`(?i)\blive +(?:music|events?|broadcasts?|shows?|performances?|audiences?|streams?|coverage|concerts?|television|radio)\b`), regexp.MustCompile(`(?i)\blive\b`), "lyve"},
}

func rewriteHomographs(text string) string {
	for _, rule := range homographs {
		text = replaceTokens(text, rule.pattern, func(phrase string) string {
			return rule.spelling.ReplaceAllStringFunc(phrase, func(word string) string {
				if word == strings.ToUpper(word) {
					return strings.ToUpper(rule.replacement)
				}
				if word[0] >= 'A' && word[0] <= 'Z' {
					return strings.ToUpper(rule.replacement[:1]) + rule.replacement[1:]
				}
				return rule.replacement
			})
		})
	}
	return text
}

// RE2 word boundaries are ASCII-only. Check adjoining runes without consuming
// delimiters, so consecutive tokens can be replaced independently.
func atTokenBoundary(text string, start, end int) bool {
	left, _ := utf8.DecodeLastRuneInString(text[:start])
	right, _ := utf8.DecodeRuneInString(text[end:])
	isWord := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || r == '_'
	}
	// Abbreviations ending in punctuation can immediately precede words.
	last, _ := utf8.DecodeLastRuneInString(text[start:end])
	return !isWord(left) && (!isWord(right) || last == '.' || last == '/')
}

func replaceTokens(text string, pattern *regexp.Regexp, expand func(string) string) string {
	matches := pattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	var out strings.Builder
	last := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		if atTokenBoundary(text, start, end) {
			out.WriteString(text[last:start])
			out.WriteString(expand(text[start:end]))
			last = end
		}
	}
	out.WriteString(text[last:])
	return out.String()
}
