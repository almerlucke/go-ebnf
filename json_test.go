package ebnf

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func jsonStringTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		return nil
	}

	value, err := strconv.Unquote(r.StringFromResult(m))
	if err != nil {
		return err
	}

	m.Result = value

	return nil
}

func jsonStringPattern() Pattern {
	hexDigit := NewAlternation(
		[]Pattern{
			NewCharacterRange('a', 'z', false, nil),
			NewCharacterRange('A', 'Z', false, nil),
			NewCharacterRange('0', '9', false, nil),
		},
		nil,
	)

	hexPattern := NewConcatenation(
		[]Pattern{
			NewTerminalString("u", nil),
			NewRepetition(hexDigit, 4, 4, nil),
		},
		nil,
	)

	escapeSequence := NewConcatenation(
		[]Pattern{
			NewTerminalString(`\`, nil),
			NewAlternation(
				[]Pattern{
					NewCharacterEnum(`"\/bfnrt`, false, nil),
					hexPattern,
				},
				nil,
			),
		},
		nil,
	)

	normalCodePoint := NewCharacterGroup(func(r rune) bool {
		return unicode.IsControl(r) || r == '\\' || r == '"'
	}, true, nil)

	return NewConcatenation(
		[]Pattern{
			NewTerminalString(`"`, nil),
			NewAny(NewAlternation([]Pattern{normalCodePoint, escapeSequence}, nil), nil),
			NewTerminalString(`"`, nil),
		},
		jsonStringTransform,
	)
}

func jsonNumberTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		return nil
	}

	value, err := strconv.ParseFloat(r.StringFromResult(m), 64)
	if err != nil {
		return err
	}

	m.Result = value

	return nil
}

func jsonNumberPattern() Pattern {
	digitw0Chars := NewCharacterRange('1', '9', false, nil)
	digitChars := NewCharacterRange('0', '9', false, nil)

	fraction := NewConcatenation(
		[]Pattern{
			NewTerminalString(".", nil),
			NewRepetition(digitChars, 0, 0, nil),
		},
		nil,
	)

	zeroOrDigits := NewAlternation(
		[]Pattern{
			NewTerminalString("0", nil),
			NewConcatenation(
				[]Pattern{
					digitw0Chars,
					NewRepetition(digitChars, 0, 0, nil),
				},
				nil,
			),
		},
		nil,
	)

	exponent := NewConcatenation(
		[]Pattern{
			NewCharacterEnum("eE", false, nil),
			NewOptional(NewCharacterEnum("-+", false, nil), nil),
			NewRepetition(digitChars, 1, 0, nil),
		},
		nil,
	)

	return NewConcatenation(
		[]Pattern{
			// Optional minus
			NewOptional(NewTerminalString("-", nil), nil),
			// 0 or digits
			zeroOrDigits,
			// Optional fraction
			NewOptional(fraction, nil),
			// Optional exponent
			NewOptional(exponent, nil),
		},
		jsonNumberTransform,
	)
}

func jsonArrayTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		return nil
	}

	values := []interface{}{}

	// Get middle alternation
	alternation := m.Result.([]*MatchResult)[1]

	// Check if not only whitespace
	if alternation.Result != nil {
		concatenation := alternation.Result.([]*MatchResult)

		// First value is not in question so just add it
		values = append(values, concatenation[0].Result)

		// Get next values (if any)
		nextValueResults := concatenation[1].Result.([]*MatchResult)
		for _, valueResult := range nextValueResults {
			// Skip comma so get index 1
			values = append(values, valueResult.Result.([]*MatchResult)[1].Result)
		}
	}

	m.Result = values

	return nil
}

func jsonArrayPattern(value Pattern, whitespace Pattern) Pattern {
	return NewConcatenation(
		[]Pattern{
			NewTerminalString("[", nil),
			NewAlternation(
				[]Pattern{
					NewConcatenation(
						[]Pattern{
							value,
							NewAny(NewConcatenation([]Pattern{NewTerminalString(",", nil), value}, nil), nil),
						},
						nil,
					),
					whitespace,
				},
				nil,
			),
			NewTerminalString("]", nil),
		},
		jsonArrayTransform,
	)
}

func jsonTrueTransform(m *MatchResult, r *Reader) error {
	if m.Match {
		m.Result = true
	}

	return nil
}

func jsonFalseTransform(m *MatchResult, r *Reader) error {
	if m.Match {
		m.Result = false
	}

	return nil
}

func jsonNullTransform(m *MatchResult, r *Reader) error {
	if m.Match {
		m.Result = nil
	}

	return nil
}

func jsonValueTransform(m *MatchResult, r *Reader) error {
	if m.Match {
		m.Result = m.Result.([]*MatchResult)[1].Result
	}

	return nil
}

func jsonWhitespaceTransform(m *MatchResult, r *Reader) error {
	if m.Match {
		m.Result = nil
	}
	return nil
}

func TestJSON(t *testing.T) {
	reader, err := NewReader(strings.NewReader(`[true, "check", [330e-2, [1, 2, 3, 4]]]`))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	whitespacePattern := NewAny(NewCharacterEnum(" \n\r\t", false, nil), jsonWhitespaceTransform)

	valueAlternation := NewAlternation(nil, nil)
	valuePattern := NewConcatenation(
		[]Pattern{
			whitespacePattern,
			valueAlternation,
			whitespacePattern,
		},
		jsonValueTransform,
	)

	arrayPattern := jsonArrayPattern(valuePattern, whitespacePattern)
	stringPattern := jsonStringPattern()
	numberPattern := jsonNumberPattern()
	truePattern := NewTerminalString("true", jsonTrueTransform)
	falsePattern := NewTerminalString("false", jsonFalseTransform)
	nullPattern := NewTerminalString("null", jsonNullTransform)

	valueAlternation.Patterns = []Pattern{truePattern, falsePattern, nullPattern, stringPattern, numberPattern, arrayPattern}

	result, err := NewConcatenation([]Pattern{valuePattern, NewEOF(nil)}, nil).Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("match: %v", result.Result.([]*MatchResult)[0].Result)
	} else {
		log.Print("no match")
	}
}
