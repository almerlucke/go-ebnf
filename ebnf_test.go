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

	ebnf := NewEBNF("", map[string]Pattern{})

	ebnf.Rules["whitespace"] = NewCharacterGroup(unicode.IsSpace)
	ebnf.Rules["visible_character"] = NewCharacterGroup(unicode.IsPrint)
	ebnf.Rules["digit"] = NewCharacterGroup(unicode.IsDigit)
	ebnf.Rules["alphabetic_character"] = NewCharacterGroup(NewCharacterGroupRangeFunction('A', 'Z'))

	ebnf.Rules["identifier"] = NewConcatenationT(identifierTransform,
		ebnf.Rules["alphabetic_character"],
		NewRepetition(0, 0, NewAlternation(ebnf.Rules["alphabetic_character"], ebnf.Rules["digit"])),
	)

	ebnf.Rules["number"] = NewRepetitionT(numberTransform, 1, 0, ebnf.Rules["digit"])

	ebnf.Rules["string"] = NewConcatenationT(stringTransform,
		NewTerminalString("\""),
		NewRepetition(0, 0, NewException(ebnf.Rules["visible_character"], NewTerminalString("\""))),
		NewTerminalString("\""),
	)

	ebnf.Rules["assignment"] = NewConcatenationT(assignmentTransform,
		ebnf.Rules["identifier"], NewTerminalString(":="), NewAlternation(ebnf.Rules["number"], ebnf.Rules["identifier"], ebnf.Rules["string"]),
	)

	ebnf.Rules["program"] = NewConcatenationT(programTransform,
		NewTerminalString("PROGRAM"), ebnf.Rules["whitespace"], ebnf.Rules["identifier"], ebnf.Rules["whitespace"],
		NewTerminalString("BEGIN"), ebnf.Rules["whitespace"],
		NewRepetition(0, 0, NewConcatenation(ebnf.Rules["assignment"], NewTerminalString(";"), ebnf.Rules["whitespace"])),
		NewTerminalString("END"),
	)

	// Assign the root rule as starting point
	ebnf.RootRule = "program"

	result, err := ebnf.Match(reader)
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

func TestLanguage(t *testing.T) {
	reader, err := NewReader(strings.NewReader(`"😃d@d\d"`))
	if err != nil {
		t.Errorf("err %v", err)
		t.FailNow()
	}

	ebnf := NewEBNF("", map[string]Pattern{})

	ebnf.Rules["quote"] = NewTerminalString(`"`)
	ebnf.Rules["backslash"] = NewTerminalString(`\`)
	ebnf.Rules["is_graphic"] = NewCharacterGroup(unicode.IsGraphic)
	ebnf.Rules["string"] = NewConcatenation(
		ebnf.Rules["quote"],
		NewRepetition(0, 0, NewAlternation(
			NewConcatenation(ebnf.Rules["backslash"], ebnf.Rules["is_graphic"]),
			NewException(ebnf.Rules["is_graphic"], ebnf.Rules["quote"]),
		)),
		ebnf.Rules["quote"],
	)
	ebnf.Rules["program"] = NewConcatenation(ebnf.Rules["string"], NewEOF())

	ebnf.RootRule = "program"

	result, err := ebnf.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("result.Result %v\n", result.Result)
	} else {
		log.Printf("no match!")
	}
}
