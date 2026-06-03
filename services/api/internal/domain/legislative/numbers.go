package legislative

import "strconv"

// parseNumberToken parses a vote count that may be written in ASCII digits,
// full-width digits, or 漢数字 (e.g. "242", "２４２", "二百四十二"). Diet counts
// stay within a few hundred, but 万 is supported for safety.
func parseNumberToken(tok string) (int, bool) {
	if tok == "" {
		return 0, false
	}
	if ascii, ok := toASCIIDigits(tok); ok {
		n, err := strconv.Atoi(ascii)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return kansujiToInt(tok)
}

// toASCIIDigits returns the ASCII form when every rune is an ASCII or full-width
// digit; ok is false if any rune is a kanji numeral.
func toASCIIDigits(s string) (string, bool) {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			out = append(out, r)
		case r >= '０' && r <= '９':
			out = append(out, '0'+(r-'０'))
		default:
			return "", false
		}
	}
	return string(out), true
}

var kansujiDigit = map[rune]int{
	'〇': 0, '零': 0, '一': 1, '二': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9,
}

var kansujiUnit = map[rune]int{'十': 10, '百': 100, '千': 1000}

// kansujiToInt converts traditional Japanese numerals up to the 万 place.
func kansujiToInt(s string) (int, bool) {
	total, section, current := 0, 0, 0
	saw := false
	for _, r := range s {
		switch {
		case r == '万':
			section = (section + current) * 10000
			total += section
			section, current = 0, 0
			saw = true
		case kansujiUnit[r] != 0:
			if current == 0 {
				current = 1
			}
			section += current * kansujiUnit[r]
			current = 0
			saw = true
		default:
			d, ok := kansujiDigit[r]
			if !ok {
				return 0, false
			}
			current = d
			saw = true
		}
	}
	if !saw {
		return 0, false
	}
	return total + section + current, true
}
