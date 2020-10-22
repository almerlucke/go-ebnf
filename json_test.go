package ebnf

import (
	"log"
	"strconv"
	"strings"
	"testing"
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

func jsonNumberTransform(m *MatchResult, r *Reader) error {
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
	digitw0Chars := NewCharacterGroup(NewCharacterGroupRangeFunction('1', '9'), false, nil)
	digitChars := NewCharacterGroup(NewCharacterGroupRangeFunction('0', '9'), false, nil)

	numberFraction := NewConcatenation(
		[]Pattern{
			NewTerminalString(".", nil),
			NewRepetition(digitChars, 0, 0, nil),
		},
		nil,
	)

	numberZeroOrDigits := NewAlternation(
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

	return NewConcatenation(
		[]Pattern{
			// Optional minus
			NewOptional(NewTerminalString("-", nil), nil),
			// 0 or digits
			numberZeroOrDigits,
			// Optional fraction
			NewOptional(numberFraction, nil),
			NewEOF(nil),
		},
		jsonNumberTransform,
	)
}

func TestJSON(t *testing.T) {
	reader, err := NewReader(strings.NewReader("-232.212"))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	// whitespaceChars := NewCharacterGroup(NewCharacterGroupEnumFunction(" \n\r\t"), false, nil)
	// whitespace := NewRepetition(whitespaceChars, 1, 0, nil)

	numberPattern := jsonNumberPattern()
	result, err := numberPattern.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("match: %v", result.Result.(*JSONValue).Value)
	} else {
		log.Print("no match")
	}
}