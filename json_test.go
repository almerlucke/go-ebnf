package ebnf

import (
	"log"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// JSONType type
type JSONType int

const (
	// JSONStringType string
	JSONStringType JSONType = iota
	// JSONNumberType string
	JSONNumberType
	// JSONObjectType string
	JSONObjectType
	// JSONArrayType string
	JSONArrayType
	// JSONTrueType string
	JSONTrueType
	// JSONFalseType string
	JSONFalseType
	// JSONNullType string
	JSONNullType
)

// JSONValue value
type JSONValue struct {
	Type  JSONType
	Value interface{}
}

func jsonStringTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		return nil
	}

	value := r.StringFromResult(m)

	m.Result = &JSONValue{
		Type:  JSONStringType,
		Value: value,
	}

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

	m.Result = &JSONValue{
		Type:  JSONNumberType,
		Value: value,
	}

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
			NewEOF(nil),
		},
		jsonNumberTransform,
	)
}

func TestJSON(t *testing.T) {
	reader, err := NewReader(strings.NewReader(`"\u0346 hallo"`))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	// whitespaceChars := NewCharacterGroup(NewCharacterGroupEnumFunction(" \n\r\t"), false, nil)
	// whitespace := NewRepetition(whitespaceChars, 1, 0, nil)

	// pattern := jsonNumberPattern()

	pattern := jsonStringPattern()
	result, err := pattern.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("match: %v", result.Result.(*JSONValue).Value)
	} else {
		log.Print("no match")
	}
}
