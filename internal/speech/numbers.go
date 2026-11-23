package speech

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	digits       = `[0-9]+`
	integer      = `(?:[0-9]{1,3}(?:,[0-9]{3})+|[0-9]+)`
	decimal      = integer + `(?:\.[0-9]+)?`
	signed       = `[+-]?` + decimal
	suffix       = `(?:st|nd|rd|th)`
	months       = `Jan(?:uary)?|Feb(?:ruary)?|Mar(?:ch)?|Apr(?:il)?|May|Jun(?:e)?|Jul(?:y)?|Aug(?:ust)?|Sep(?:t(?:ember)?)?|Oct(?:ober)?|Nov(?:ember)?|Dec(?:ember)?`
	meridiem     = `(?i:[ap]\.?m)\b\.?`
	numericDate  = `[0-9]{1,4}[-/][0-9]{1,2}[-/][0-9]{1,4}`
	numericClock = `[0-9]{1,2}:[0-9]{2}`
)

var measure = `(?:%|` + unitPattern() + `)`

// Recognize compound expressions before their components. Each token is
// expanded once, so a date can't be partially rewritten as a fraction.
var numericRules = []struct {
	pattern string
	speak   func(string) string
}{
	{`[0-9]+(?:\.[0-9]+){2,}`, unchanged},
	{`(?:\+?1[-. ]?)?(?:\([0-9]{3}\)[-. ]?|[0-9]{3}[-. ])[0-9]{3}[-. ][0-9]{4}`, speakPhone},
	{numericDate, speakNumericDate},
	{`(?i:` + months + `)\.? +[0-9]{1,2}` + suffix + `?\b(?:,? +[0-9]{4}\b)?`, speakNamedDate},
	{numericClock + `(?::[0-9]{2}| *` + meridiem + `)?|[0-9]{1,2} *` + meridiem, speakTime},
	{digits + `/\s*` + digits, speakFraction},
	{`[+-]?[$€£¥] *` + decimal, speakCurrency},
	{digits + suffix, speakOrdinal},
	{signed + ` *[–—-] *` + signed + `(?: *` + measure + `)?`, speakRange},
	{signed + ` *` + measure, speakQuantity},
	{signed, speakNumber},
}

var numericPattern = func() *regexp.Regexp {
	patterns := make([]string, len(numericRules))
	for i, rule := range numericRules {
		patterns[i] = "(" + rule.pattern + ")"
	}
	return regexp.MustCompile(strings.Join(patterns, "|"))
}()

func speakNumericTokens(text string) string {
	var out strings.Builder
	last := 0
	for _, match := range numericPattern.FindAllStringSubmatchIndex(text, -1) {
		start, end := match[0], match[1]
		if !atTokenBoundary(text, start, end) {
			continue
		}
		for i, rule := range numericRules {
			if match[2+2*i] == start {
				out.WriteString(text[last:start])
				out.WriteString(rule.speak(text[start:end]))
				last = end
				break
			}
		}
	}
	if last == 0 {
		return text
	}
	out.WriteString(text[last:])
	return out.String()
}

func unchanged(token string) string { return token }

var small = [...]string{
	"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
	"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen",
	"seventeen", "eighteen", "nineteen",
}
var tens = [...]string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
var scales = [...]string{"", "thousand", "million", "billion", "trillion", "quadrillion", "quintillion"}

func cardinal(n int) string {
	switch {
	case n < 20:
		return small[n]
	case n < 100:
		word := tens[n/10]
		if n%10 != 0 {
			word += "-" + small[n%10]
		}
		return word
	case n < 1000:
		word := small[n/100] + " hundred"
		if n%100 != 0 {
			word += " and " + cardinal(n%100)
		}
		return word
	default:
		scale, factor := 1, 1000
		for n/factor >= 1000 {
			scale++
			factor *= 1000
		}
		word := cardinal(n/factor) + " " + scales[scale]
		if rest := n % factor; rest != 0 {
			if rest < 100 {
				word += " and " + cardinal(rest)
			} else {
				word += ", " + cardinal(rest)
			}
		}
		return word
	}
}

var irregularOrdinals = map[string]string{
	"one": "first", "two": "second", "three": "third", "five": "fifth",
	"eight": "eighth", "nine": "ninth", "twelve": "twelfth",
}

func ordinal(n int) string {
	words := cardinal(n)
	pos := strings.LastIndexAny(words, " -") + 1
	word := words[pos:]
	if replacement, ok := irregularOrdinals[word]; ok {
		return words[:pos] + replacement
	}
	if strings.HasSuffix(word, "y") {
		return strings.TrimSuffix(words, "y") + "ieth"
	}
	return words + "th"
}

func speakDigits(token, sep string) string {
	var words []string
	for _, digit := range token {
		if digit >= '0' && digit <= '9' {
			words = append(words, small[digit-'0'])
		}
	}
	return strings.Join(words, sep)
}

func speakNumber(token string) string {
	raw := strings.ReplaceAll(token, ",", "")
	sign := ""
	if raw[0] == '-' {
		sign, raw = "minus ", raw[1:]
	} else if raw[0] == '+' {
		sign, raw = "plus ", raw[1:]
	}
	whole, decimal, hasPoint := strings.Cut(raw, ".")
	if len(whole) > 1 && whole[0] == '0' && !hasPoint {
		return sign + speakDigits(whole, " ")
	}
	n, err := strconv.Atoi(whole)
	if err != nil {
		return token
	}
	words := sign + cardinal(n)
	if hasPoint {
		words += " point " + speakDigits(decimal, " ")
	}
	return words
}

var rangePattern = regexp.MustCompile(`^(` + signed + `) *[–—-] *(` + signed + `)(?: *` + measure + `)?$`)

func speakRange(token string) string {
	parts := rangePattern.FindStringSubmatchIndex(token)
	first, last := token[parts[2]:parts[3]], token[parts[4]:parts[5]]
	spoken := speakNumber(last)
	if parts[5] < len(token) {
		spoken = speakQuantity(token[parts[4]:])
	}
	return speakNumber(first) + " to " + spoken
}

func speakOrdinal(token string) string {
	n, err := strconv.Atoi(token[:len(token)-2])
	if err != nil {
		return token
	}
	return ordinal(n)
}

func speakPhone(token string) string {
	spoken := speakDigits(token, ", ")
	if strings.HasPrefix(token, "+") {
		spoken = "plus " + spoken
	}
	return spoken
}

func speakFraction(token string) string {
	numerator, denominator, _ := strings.Cut(token, "/")
	n, nErr := strconv.Atoi(numerator)
	d, dErr := strconv.Atoi(strings.TrimSpace(denominator))
	if nErr != nil || dErr != nil || d == 0 {
		return token
	}
	word := ordinal(d)
	switch d {
	case 2:
		if n == 1 {
			word = "half"
		} else {
			word = "halves"
		}
	case 4:
		word = "quarter"
	}
	if n != 1 && d != 2 {
		word += "s"
	}
	return cardinal(n) + " " + word
}

func speakNumericDate(token string) string {
	sep := "-"
	if strings.Contains(token, "/") {
		sep = "/"
	}
	parts := strings.Split(token, sep)
	if len(parts) != 3 {
		return token
	}
	if len(parts[0]) == 4 {
		parts[0], parts[1], parts[2] = parts[1], parts[2], parts[0]
	}
	if len(parts[2]) != 4 {
		return token
	}
	month, _ := strconv.Atoi(parts[0])
	day, _ := strconv.Atoi(parts[1])
	year, _ := strconv.Atoi(parts[2])
	return speakDate(token, time.Month(month), day, year)
}

func speakNamedDate(token string) string {
	parts := strings.Fields(strings.ReplaceAll(token, ",", " "))
	monthName := strings.ToLower(parts[0][:3])
	day, _ := strconv.Atoi(strings.TrimRight(parts[1], "stndrh"))
	year := 0
	if len(parts) == 3 {
		year, _ = strconv.Atoi(parts[2])
	}
	for month := time.January; month <= time.December; month++ {
		if strings.EqualFold(month.String()[:3], monthName) {
			return speakDate(token, month, day, year)
		}
	}
	return token
}

func speakDate(token string, month time.Month, day, year int) string {
	if date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC); date.Month() != month || date.Day() != day {
		return token
	}
	spoken := month.String() + " " + ordinal(day)
	if year != 0 {
		spoken += ", " + cardinal(year)
	}
	return spoken
}

var timePattern = regexp.MustCompile(`^([0-9]{1,2})(?::([0-9]{2}))? *(?:((?i:[ap]))\.?m\.?)?$`)

func speakTime(token string) string {
	parts := timePattern.FindStringSubmatch(strings.ToLower(token))
	if parts == nil {
		return token
	}
	hour, _ := strconv.Atoi(parts[1])
	minute, _ := strconv.Atoi(parts[2])
	meridiem := strings.ToUpper(parts[3])
	if hour > 23 || minute > 59 || (meridiem != "" && (hour == 0 || hour > 12)) {
		return token
	}
	spoken := cardinal(hour)
	switch {
	case minute == 0:
		spoken += " o'clock"
	case minute < 10:
		spoken += " oh " + cardinal(minute)
	default:
		spoken += " " + cardinal(minute)
	}
	if meridiem != "" {
		spoken += " " + meridiem + "M"
	}
	if strings.HasSuffix(token, ".") {
		spoken += "."
	}
	return spoken
}

type wordForms struct {
	singular, plural string
}

var currencies = map[rune][2]wordForms{
	'$': {{"dollar", "dollars"}, {"cent", "cents"}},
	'€': {{"euro", "euros"}, {"cent", "cents"}},
	'£': {{"pound", "pounds"}, {"penny", "pence"}},
	'¥': {{"yen", "yen"}, {"sen", "sen"}},
}

func speakCurrency(token string) string {
	sign := ""
	raw := token
	if raw[0] == '-' {
		sign, raw = "minus ", raw[1:]
	} else if raw[0] == '+' {
		sign, raw = "plus ", raw[1:]
	}
	symbol, size := utf8.DecodeRuneInString(raw)
	amount := strings.TrimSpace(raw[size:])
	names := currencies[symbol]
	whole, fraction, _ := strings.Cut(strings.ReplaceAll(amount, ",", ""), ".")
	major, err := strconv.Atoi(whole)
	if err != nil || (len(fraction) > 2 && strings.Trim(fraction[2:], "0") != "") {
		return token
	}
	minor, _ := strconv.Atoi((fraction + "00")[:2])
	var parts []string
	if major != 0 || minor == 0 {
		parts = append(parts, cardinal(major)+" "+names[0].forCount(major == 1))
	}
	if minor != 0 {
		parts = append(parts, cardinal(minor)+" "+names[1].forCount(minor == 1))
	}
	return sign + strings.Join(parts, " and ")
}

func (forms wordForms) forCount(singular bool) string {
	if singular {
		return forms.singular
	}
	return forms.plural
}

var units = map[string]wordForms{
	"km": {"kilometer", "kilometers"}, "mi": {"mile", "miles"}, "m": {"meter", "meters"},
	"cm": {"centimeter", "centimeters"}, "mm": {"millimeter", "millimeters"},
	"ft": {"foot", "feet"}, "in": {"inch", "inches"}, "yd": {"yard", "yards"},
	"kg": {"kilogram", "kilograms"}, "g": {"gram", "grams"}, "mg": {"milligram", "milligrams"},
	"lb": {"pound", "pounds"}, "lbs": {"pound", "pounds"}, "oz": {"ounce", "ounces"},
	"L": {"liter", "liters"}, "mL": {"milliliter", "milliliters"}, "ml": {"milliliter", "milliliters"},
	"gal": {"gallon", "gallons"}, "fl oz": {"fluid ounce", "fluid ounces"},
	"°F": {"degree Fahrenheit", "degrees Fahrenheit"}, "°C": {"degree Celsius", "degrees Celsius"},
	"°K": {"kelvin", "kelvins"}, "mph": {"mile per hour", "miles per hour"},
	"kph": {"kilometer per hour", "kilometers per hour"}, "km/h": {"kilometer per hour", "kilometers per hour"},
	"m/s": {"meter per second", "meters per second"},
	"KB":  {"kilobyte", "kilobytes"}, "MB": {"megabyte", "megabytes"}, "GB": {"gigabyte", "gigabytes"}, "TB": {"terabyte", "terabytes"},
	"Kb": {"kilobit", "kilobits"}, "Mb": {"megabit", "megabits"}, "Gb": {"gigabit", "gigabits"},
	"ms": {"millisecond", "milliseconds"}, "μs": {"microsecond", "microseconds"}, "ns": {"nanosecond", "nanoseconds"},
	"MHz": {"megahertz", "megahertz"}, "GHz": {"gigahertz", "gigahertz"}, "kHz": {"kilohertz", "kilohertz"}, "Hz": {"hertz", "hertz"},
	"kW": {"kilowatt", "kilowatts"}, "MW": {"megawatt", "megawatts"}, "W": {"watt", "watts"},
	"V": {"volt", "volts"}, "A": {"amp", "amps"}, "%": {"percent", "percent"},
}

func unitPattern() string {
	keys := make([]string, 0, len(units))
	for key := range units {
		if key != "%" {
			keys = append(keys, regexp.QuoteMeta(key))
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	// Require a token boundary after units so "5 messages" reads as ordinary prose.
	return `(?:` + strings.Join(keys, "|") + `)\b`
}

var quantityPattern = regexp.MustCompile(`^(` + signed + `)( *)(.+)$`)

func speakQuantity(token string) string {
	parts := quantityPattern.FindStringSubmatch(token)
	number, space, unit := parts[1], parts[2], parts[3]
	if unit == "in" && space != "" {
		return speakNumber(number) + space + unit
	}
	value, _ := strconv.ParseFloat(strings.ReplaceAll(number, ",", ""), 64)
	return speakNumber(number) + " " + units[unit].forCount(value == 1 || value == -1)
}
