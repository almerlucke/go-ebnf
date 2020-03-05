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
	reader := NewReader(strings.NewReader("PROGRAM DEMO1\nBEGIN\nA:=\"testa\";\nTESTAL:=12234;\nEND"))
	ebnf := NewEBNF()

	ebnf.Rules["whitespace"] = &CharacterGroup{Group: unicode.IsSpace}
	ebnf.Rules["visible_character"] = &CharacterGroup{Group: unicode.IsPrint}
	ebnf.Rules["digit"] = &CharacterGroup{Group: unicode.IsDigit}
	ebnf.Rules["alphabetic_character"] = &CharacterRange{Low: 'A', High: 'Z'}

	ebnf.Rules["identifier"] = &Concatenation{
		T: identifierTransform,
		Patterns: []Pattern{
			ebnf.Rules["alphabetic_character"],
			&Repetition{
				Min: 0,
				Max: 0,
				Pattern: &Alternation{
					Patterns: []Pattern{
						ebnf.Rules["alphabetic_character"],
						ebnf.Rules["digit"],
					},
				},
			},
		},
	}

	ebnf.Rules["number"] = &Repetition{
		T:       numberTransform,
		Min:     1,
		Max:     0,
		Pattern: ebnf.Rules["digit"],
	}

	ebnf.Rules["string"] = &Concatenation{
		T: stringTransform,
		Patterns: []Pattern{
			&TerminalString{String: "\""},
			&Repetition{
				Min: 0,
				Max: 0,
				Pattern: &Exception{
					MustMatch: ebnf.Rules["visible_character"],
					Except:    &TerminalString{String: "\""},
				},
			},
			&TerminalString{String: "\""},
		},
	}

	ebnf.Rules["assignment"] = &Concatenation{
		T: assignmentTransform,
		Patterns: []Pattern{
			ebnf.Rules["identifier"],
			&TerminalString{String: ":="},
			&Alternation{
				Patterns: []Pattern{
					ebnf.Rules["number"],
					ebnf.Rules["identifier"],
					ebnf.Rules["string"],
				},
			},
		},
	}

	ebnf.Rules["program"] = &Concatenation{
		T: programTransform,
		Patterns: []Pattern{
			&TerminalString{String: "PROGRAM"},
			ebnf.Rules["whitespace"],
			ebnf.Rules["identifier"],
			ebnf.Rules["whitespace"],
			&TerminalString{String: "BEGIN"},
			ebnf.Rules["whitespace"],
			&Repetition{
				Min: 0,
				Max: 0,
				Pattern: &Concatenation{
					Patterns: []Pattern{
						ebnf.Rules["assignment"],
						&TerminalString{String: ";"},
						ebnf.Rules["whitespace"],
					},
				},
			},
			&TerminalString{String: "END"},
		},
	}

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
