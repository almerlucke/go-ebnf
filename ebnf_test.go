package ebnf

import (
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
	var builder strings.Builder
	charResults := m.Result.([]*MatchResult)

	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()

	return nil
}

func stringTransform(m *MatchResult, r *Reader) error {
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
	log.Printf("assignment: %v - %v\n", *m.BeginPos, *m.EndPos)

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
	alphabeticCharacter := NewCharacterGroup(NewCharacterGroupRangeFunction('A', 'Z'), false, nil)
	identifier := NewConcatenation(
		[]Pattern{
			alphabeticCharacter,
			NewRepetition(
				NewAlternation([]Pattern{alphabeticCharacter, digit}, nil), 0, 0, nil,
			),
		},
		identifierTransform,
	)
	number := NewRepetition(digit, 1, 0, numberTransform)
	stringRule := NewConcatenation(
		[]Pattern{
			NewTerminalString("\"", nil),
			NewRepetition(NewException(visibleCharacter, NewTerminalString("\"", nil), nil), 0, 0, nil),
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
			NewRepetition(
				NewConcatenation([]Pattern{assignment, NewTerminalString(";", nil), whitespace}, nil), 0, 0, nil,
			),
			NewTerminalString("END", nil),
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
		log.Printf("no match\n")
	}
}

func complexStringBackslashTransform(m *MatchResult, r *Reader) error {
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
