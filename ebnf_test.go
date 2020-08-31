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

func identifierTransform(m *MatchResult) {
	params := m.Result.([]*MatchResult)
	var builder strings.Builder

	builder.WriteString(params[0].Result.(string))

	charResults := params[1].Result.([]*MatchResult)
	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()
}

func numberTransform(m *MatchResult) {
	var builder strings.Builder
	charResults := m.Result.([]*MatchResult)

	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()
}

func stringTransform(m *MatchResult) {
	var builder strings.Builder
	params := m.Result.([]*MatchResult)

	charResults := params[1].Result.([]*MatchResult)
	for _, m := range charResults {
		builder.WriteString(m.Result.(string))
	}

	m.Result = builder.String()
}

func assignmentTransform(m *MatchResult) {
	log.Printf("assignment: %v - %v\n", *m.BeginPos, *m.EndPos)

	params := m.Result.([]*MatchResult)
	identifier := params[0].Result.(string)
	value := params[2].Result.(string)

	m.Result = &assignment{
		Identifier: identifier,
		Value:      value,
	}
}

func programTransform(m *MatchResult) {
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
}

func TestEBNF(t *testing.T) {
	reader, err := NewReader(strings.NewReader("PROGRAM DEMO12\nBEGIN\nAB:=\"testsa 123!!!\";\nTESTAR:=1772234;\nEND"))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	whitespace := NewRepetition(1, 0, NewCharacterGroup(unicode.IsSpace, false))
	visibleCharacter := NewCharacterGroup(unicode.IsPrint, false)
	digit := NewCharacterGroup(unicode.IsDigit, false)
	alphabeticCharacter := NewCharacterGroup(NewCharacterGroupRangeFunction('A', 'Z'), false)
	identifier := NewConcatenationT(identifierTransform,
		alphabeticCharacter,
		NewRepetition(0, 0, NewAlternation(alphabeticCharacter, digit)),
	)
	number := NewRepetitionT(numberTransform, 1, 0, digit)
	stringRule := NewConcatenationT(stringTransform,
		NewTerminalString("\""),
		NewRepetition(0, 0, NewException(visibleCharacter, NewTerminalString("\""))),
		NewTerminalString("\""),
	)

	assignment := NewConcatenationT(assignmentTransform,
		identifier, NewTerminalString(":="), NewAlternation(number, identifier, stringRule),
	)

	programRule := NewConcatenationT(
		programTransform,
		NewTerminalString("PROGRAM"), whitespace, identifier, whitespace,
		NewTerminalString("BEGIN"), whitespace,
		NewRepetition(0, 0, NewConcatenation(assignment, NewTerminalString(";"), whitespace)),
		NewTerminalString("END"),
	)

	result, err := programRule.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		program := result.Result.(*program)

		log.Printf("program name %v\n", program.Identifier)

		for _, assignment := range program.Assignments {
			log.Printf("assignment identifier: %v = %v\n", assignment.Identifier, assignment.Value)
		}
	} else {
		log.Printf("no match\n")
	}
}

func complexStringBackslashTransform(m *MatchResult) {
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
}

func complexStringTransform(m *MatchResult) {
	stringBaseElements := m.Result.([]*MatchResult)
	stringRepeatedElements := stringBaseElements[1].Result.([]*MatchResult)

	var builder strings.Builder

	for _, element := range stringRepeatedElements {
		builder.WriteString(element.Result.(string))
	}

	m.Result = builder.String()

	log.Printf("string: %v\n", m.Result)
}

func TestLanguage(t *testing.T) {
	reader, err := NewReader(strings.NewReader(`"😃d@d\td"`))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	quoteRule := NewTerminalString(`"`)
	backslashRule := NewTerminalString(`\`)
	isGraphicRule := NewCharacterGroup(unicode.IsGraphic, false)
	stringRule := NewConcatenationT(
		complexStringTransform,
		quoteRule,
		NewRepetition(0, 0, NewAlternation(
			NewConcatenationT(complexStringBackslashTransform, backslashRule, isGraphicRule),
			NewException(isGraphicRule, quoteRule),
		)),
		quoteRule,
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
