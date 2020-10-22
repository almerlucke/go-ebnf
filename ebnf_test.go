package ebnf

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"unicode"
)

/**
*
*	Test transform functionality
*
**/

type assignment struct {
	Identifier string
	Value      string
}

type program struct {
	Identifier  string
	Assignments []*assignment
}

func identifierTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		m.Error = errors.New("expected identifier")
		return nil
	}

	params := m.Result.([]*MatchResult)
	var builder strings.Builder

	builder.WriteString(params[0].Result.(string))

	charResults := params[1].Result.([]*MatchResult)
	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()

	return nil
}

func numberTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		m.Error = errors.New("expected number")
		return nil
	}

	var builder strings.Builder
	charResults := m.Result.([]*MatchResult)

	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()

	return nil
}

func stringTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		m.Error = errors.New("expected string")
		return nil
	}

	var builder strings.Builder
	params := m.Result.([]*MatchResult)

	charResults := params[1].Result.([]*MatchResult)
	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()

	return nil
}

func assignmentTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		m.Error = errors.New("invalid assignment")
		return nil
	}

	params := m.Result.([]*MatchResult)
	identifier := params[0].Result.(string)
	value := params[2].Result.(string)

	m.Result = &assignment{
		Identifier: identifier,
		Value:      value,
	}

	return nil
}

func programTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		m.Error = errors.New("invalid program")
		return nil
	}

	program := &program{}

	params := m.Result.([]*MatchResult)
	program.Identifier = params[2].Result.(string)

	assignments := []*assignment{}
	assignmentResults := params[6].Result.([]*MatchResult)

	for _, assignmentResult := range assignmentResults {
		assignment := assignmentResult.Result.([]*MatchResult)[0].Result.(*assignment)
		assignments = append(assignments, assignment)
	}

	program.Assignments = assignments

	m.Result = program

	return nil
}

func TestEBNF(t *testing.T) {
	reader, err := NewReader(strings.NewReader("PROGRAM DEMO12\nBEGIN\nAB:=\"testsa 123!!!\";\nTESTAR:=1772234;\nEND"))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	whitespace := NewRepetition(NewCharacterGroup(unicode.IsSpace, false, nil), 1, 0, nil)
	visibleCharacter := NewCharacterGroup(unicode.IsPrint, false, nil)
	digit := NewCharacterGroup(unicode.IsDigit, false, nil)
	alphabeticCharacter := NewCharacterRange('A', 'Z', false, nil)
	identifier := NewConcatenation(
		[]Pattern{
			alphabeticCharacter,
			NewAny(NewAlternation([]Pattern{alphabeticCharacter, digit}, nil), nil),
		},
		identifierTransform,
	)
	number := NewRepetition(digit, 1, 0, numberTransform)
	stringRule := NewConcatenation(
		[]Pattern{
			NewTerminalString("\"", nil),
			NewAny(NewException(visibleCharacter, NewTerminalString("\"", nil), nil), nil),
			NewTerminalString("\"", nil),
		},
		stringTransform,
	)

	assignment := NewConcatenation(
		[]Pattern{
			identifier, NewTerminalString(":=", nil), NewAlternation([]Pattern{number, identifier, stringRule}, nil),
		},
		assignmentTransform,
	)

	programRule := NewConcatenation(
		[]Pattern{
			NewTerminalString("PROGRAM", nil), whitespace, identifier, whitespace,
			NewTerminalString("BEGIN", nil), whitespace,
			NewAny(
				NewConcatenation([]Pattern{assignment, NewTerminalString(";", nil), whitespace}, nil), nil,
			),
			NewTerminalString("END", func(m *MatchResult, r *Reader) error {
				if !m.Match {
					m.Error = fmt.Errorf("expected END statement line %d - pos %d", m.BeginPos.linePos+1, m.BeginPos.relativeCharPos+1)
				}
				return nil
			}),
		},
		programTransform,
	)

	result, err := programRule.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		program := result.Result.(*program)

		log.Printf("program name %v - start %v, end %v \n", program.Identifier, *result.BeginPos, *result.EndPos)

		for _, assignment := range program.Assignments {
			log.Printf("assignment identifier: %v = %v\n", assignment.Identifier, assignment.Value)
		}
	} else {
		err = result.Error
		for err != nil {
			log.Printf("err: %v\n", err)
			err = nil
			if result.Result != nil {
				result = result.Result.(*MatchResult)
				err = result.Error
			}
		}
	}
}

func complexStringBackslashTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		return nil
	}

	backslashElements := m.Result.([]*MatchResult)
	escapedChar := backslashElements[1].Result.(string)

	if escapedChar == "n" {
		escapedChar = "\n"
	} else if escapedChar == "t" {
		escapedChar = "\t"
	} else if escapedChar == "r" {
		escapedChar = "\r"
	}

	m.Result = escapedChar

	return nil
}

func complexStringTransform(m *MatchResult, r *Reader) error {
	if !m.Match {
		return nil
	}

	stringBaseElements := m.Result.([]*MatchResult)
	stringRepeatedElements := stringBaseElements[1].Result.([]*MatchResult)

	var builder strings.Builder

	for _, element := range stringRepeatedElements {
		builder.WriteString(element.Result.(string))
	}

	m.Result = builder.String()

	log.Printf("string: %v\n", m.Result)

	return nil
}

func TestLanguage(t *testing.T) {
	reader, err := NewReader(strings.NewReader(`"😃d@d\td"`))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	quoteRule := NewTerminalString(`"`, nil)
	backslashRule := NewTerminalString(`\`, nil)
	isGraphicRule := NewCharacterGroup(unicode.IsGraphic, false, nil)
	stringRule := NewConcatenation(
		[]Pattern{
			quoteRule,
			NewRepetition(
				NewAlternation(
					[]Pattern{
						NewConcatenation(
							[]Pattern{backslashRule, isGraphicRule},
							complexStringBackslashTransform,
						),
						NewException(isGraphicRule, quoteRule, nil),
					},
					nil,
				), 0, 0, nil,
			),
			quoteRule,
		},
		complexStringTransform,
	)

	result, err := stringRule.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("result.Result %v\n", result.Result)
	} else {
		log.Printf("no match!")
	}
}
