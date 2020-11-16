package ebnf

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
	"unicode"
)

func jsonStringTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		if m.PartialMatch {
			m.Error = errors.New("not a valid string")
			r.PushError(m)
		}
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
	digitno0Chars := NewCharacterRange('1', '9', false, nil)
	digitChars := NewCharacterRange('0', '9', false, nil)

	zeroOrDigits := NewAlternation(
		[]Pattern{
			NewTerminalString("0", nil),
			NewConcatenation(
				[]Pattern{
					digitno0Chars,
					NewRepetition(digitChars, 0, 0, nil),
				},
				nil,
			),
		},
		nil,
	)

	fraction := NewConcatenation(
		[]Pattern{
			NewTerminalString(".", nil),
			NewRepetition(digitChars, 0, 0, nil),
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
			NewOptional(NewTerminalString("-", nil), nil),
			zeroOrDigits,
			NewOptional(fraction, nil),
			NewOptional(exponent, nil),
		},
		jsonNumberTransform,
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
	} else {
		m.Error = errors.New("no valid json value found")
	}

	return nil
}

func jsonWhitespaceTransform(m *MatchResult, r *Reader) error {
	if m.Match {
		m.Result = nil
	}
	return nil
}

func jsonArrayTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		if m.PartialMatch {
			m.Error = fmt.Errorf("array not closed properly")
			r.PushError(m)
		}
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
	oneOrManyValues := NewConcatenation(
		[]Pattern{
			value,
			NewAny(NewConcatenation([]Pattern{NewTerminalString(",", nil), value}, nil), nil),
		},
		nil,
	)

	return NewConcatenation(
		[]Pattern{
			NewTerminalString("[", nil),
			NewAlternation(
				[]Pattern{
					oneOrManyValues,
					// Sometimes order matter, whitespace needs to be after oneOrManyValues
					whitespace,
				},
				nil,
			),
			NewTerminalString("]", nil),
		},
		jsonArrayTransform,
	)
}

func jsonObjectTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		if m.PartialMatch {
			m.Error = errors.New("object not properly closed")
			r.PushError(m)
		}
		return nil
	}

	getKey := func(keyResult *MatchResult) string {
		return keyResult.Result.([]*MatchResult)[0].Result.([]*MatchResult)[1].Result.(string)
	}

	getValue := func(keyResult *MatchResult) interface{} {
		return keyResult.Result.([]*MatchResult)[2].Result
	}

	object := map[string]interface{}{}

	// Get middle alternation
	alternation := m.Result.([]*MatchResult)[1]

	// Check if not only whitespace
	if alternation.Result != nil {
		concatenation := alternation.Result.([]*MatchResult)

		// First value is not in question so just add it
		keyValue := concatenation[0]
		object[getKey(keyValue)] = getValue(keyValue)

		// Get next values (if any)
		nextValueResults := concatenation[1].Result.([]*MatchResult)
		for _, valueResult := range nextValueResults {
			// Skip comma so get index 1
			keyValue := valueResult.Result.([]*MatchResult)[1]
			object[getKey(keyValue)] = getValue(keyValue)
		}
	}

	m.Result = object

	return nil
}

func jsonObjectPattern(value Pattern, str Pattern, whitespace Pattern) Pattern {
	key := NewConcatenation([]Pattern{whitespace, str, whitespace}, nil)
	keyValue := NewConcatenation([]Pattern{key, NewTerminalString(":", nil), value}, nil)
	oneOrManyKeyValues := NewConcatenation(
		[]Pattern{
			keyValue,
			NewAny(NewConcatenation([]Pattern{NewTerminalString(",", nil), keyValue}, nil), nil),
		},
		nil,
	)

	return NewConcatenation(
		[]Pattern{
			NewTerminalString("{", nil),
			NewAlternation(
				[]Pattern{
					oneOrManyKeyValues,
					// Sometimes order matter, whitespace needs to be after oneOrManyKeyValues
					whitespace,
				},
				nil,
			),
			NewTerminalString("}", nil),
		},
		jsonObjectTransform,
	)
}

func TestJSON(t *testing.T) {
	file, err := os.Open("test2.json")
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	defer file.Close()

	// strings.NewReader(`{"a" : 1, "b" : 2}`)

	reader, err := NewReader(bufio.NewReader(file))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	whitespacePattern := NewAny(NewCharacterEnum(" \n\r\t", false, nil), jsonWhitespaceTransform)

	valueAlternation := NewAlternation(nil, nil)
	valuePattern := NewConcatenation(
		[]Pattern{whitespacePattern, valueAlternation, whitespacePattern}, jsonValueTransform,
	)

	stringPattern := jsonStringPattern()
	arrayPattern := jsonArrayPattern(valuePattern, whitespacePattern)
	objectPattern := jsonObjectPattern(valuePattern, stringPattern, whitespacePattern)
	numberPattern := jsonNumberPattern()
	truePattern := NewTerminalString("true", jsonTrueTransform)
	falsePattern := NewTerminalString("false", jsonFalseTransform)
	nullPattern := NewTerminalString("null", jsonNullTransform)

	valueAlternation.Patterns = []Pattern{
		stringPattern, numberPattern, objectPattern, arrayPattern, truePattern, falsePattern, nullPattern,
	}

	result, err := NewConcatenation([]Pattern{valuePattern, NewEOF(nil)}, nil).Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("match: %v", result.Result.([]*MatchResult)[0].Result)
	} else {
		deepest := reader.DeepestError()
		if deepest.Error != nil {
			log.Printf("err result: %v %v\n", deepest.Error, deepest.RangeString())
		}
	}
}
