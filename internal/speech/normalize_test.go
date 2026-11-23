package speech

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"punctuation", "“ＦＯＲＥＷＯＲＤ” — soft\u00adhyphen!! Wait …", `"Foreword", softhyphen! Wait...`},
		{"addresses", "See https://example.org/news. Email first.last+3@ex-ample.org or @news; #Updates!", "See link. Email first dot last plus three at ex dash ample dot org or at news; Updates!"},
		{"abbreviations", "Dr. Lee & Prof. Fox wrote FAQ w/o notes, e.g. ASAP.", "Doctor Lee and Professor Fox wrote F A Q without notes, for example as soon as possible."},
		{"headings and acronyms", "CHAPTER XVIII: NASA, NFB, HTML AND PDF", "Chapter eighteenth: NASA, N F B, H T M L AND P D F"},
		{"phone", "Call +1 (312) 555-0102.", "Call plus one, three, one, two, five, five, five, zero, one, zero, two."},
		{"fractions", "1/2, 3/4, 2/7, 2½, and 3/0.", "one half, three quarters, two sevenths, two and one half, and 3/0."},
		{"units", "1 m/s, 12km/h, 0.5 μs, 5–7%, 1 ft and 86℉.", "one meter per second, twelve kilometers per hour, zero point five microseconds, five to seven percent, one foot and eighty-six degrees Fahrenheit."},
		{"currency", "$1.50, $0.02, £0.01, €1,234.05 and −¥5.", "one dollar and fifty cents, two cents, one penny, one thousand, two hundred and thirty-four euros and five cents and minus five yen."},
		{"numbers", "-12.30, 004, 21st, 40th, 5–7 and 2,345.", "minus twelve point three zero, zero zero four, twenty-first, fortieth, five to seven and two thousand, three hundred and forty-five."},
		{"dates", "Jan. 2, 2026, 11/23/2026, May 1st and 2024-02-29.", "January second, two thousand and twenty-six, November twenty-third, two thousand and twenty-six, May first and February twenty-ninth, two thousand and twenty-four."},
		{"times", "At 7:05 p.m., 9 am, 12:00 or 16:30.", "At seven oh five PM, nine o'clock AM, twelve o'clock or sixteen thirty."},
		{"homographs", "I have read it. Tear apart the box. Wind the clock. Minute details. Live music and bass guitar.", "I have red it. Tare apart the box. Wynd the clock. Mynoot details. Lyve music and base guitar."},
		{"ambiguity", "Read about a tear, high wind, a minute, live fish and sea bass. There were 5 in the room.", "Read about a tear, high wind, a minute, live fish and sea bass. There were five in the room."},
		{"nonmatches", "ÉPDF, H2O, v1.2.3, 1.2.3, 15:99, 02/30/2026 and March 99.", "ÉPDF, H2O, v1.2.3, 1.2.3, 15:99, 02/30/2026 and March 99."},
		{"literal markup and breaks", "2 < 3\n\nUse <tag> and [text].", "two < three\n\nUse <tag> and [text]."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.input); got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSupportsLanguage(t *testing.T) {
	for _, tag := range []string{"", "en", "eng", "en-US", "en_GB", "English"} {
		if !SupportsLanguage(tag) {
			t.Errorf("should support %q", tag)
		}
	}
	for _, tag := range []string{"fr", "es-MX", "und", "invalid"} {
		if SupportsLanguage(tag) {
			t.Errorf("should skip %q", tag)
		}
	}
}
