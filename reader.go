package ebnf

import (
	"bufio"
	"io"
)

// ReaderPos holds character and line positions
type ReaderPos struct {
	absoluteCharPos int
	relativeCharPos int
	linePos         int
}

// Reader buffers runes to allow us to backtrack when the runes do not match a pattern
type Reader struct {
	buf          []rune
	bufPos       int
	bufPosEnd    int
	bufPosStack  []int
	lines        []int
	linePos      int
	linePosEnd   int
	linePosStack []int
	errorStack   []*MatchResult
}

// NewReader creates a new reader, all runes in input reader are first read and buffered
func NewReader(r io.Reader) (*Reader, error) {
	rr := bufio.NewReader(r)
	rs := []rune{}

	// Prefetch all runes
	for {
		r, _, err := rr.ReadRune()
		if err != nil {
			if err != io.EOF {
				return nil, err
			}

			break
		}

		rs = append(rs, r)
	}

	// Prefetch all lines, normalize CRLF sequences to LF
	lines := []int{}
	l := len(rs)
	index := 0

	for index < l {
		r := rs[index]

		index++

		if r == '\r' {
			if index < l && rs[index] == '\n' {
				index++
			}

			lines = append(lines, index)
		} else if r == '\n' {
			lines = append(lines, index)
		}
	}

	// Create reader with buffer and lines
	return &Reader{
		buf:          rs,
		bufPosEnd:    len(rs),
		bufPosStack:  []int{0},
		linePosStack: []int{0},
		lines:        lines,
		linePosEnd:   len(lines),
		errorStack:   []*MatchResult{},
	}, nil
}

// Relative position of cursor with regards to line position
func (r *Reader) relativePosition() int {
	if r.linePos == 0 {
		return r.bufPos
	}

	return r.bufPos - r.lines[r.linePos-1]
}

// CurrentPosition returns the current reader position
func (r *Reader) CurrentPosition() *ReaderPos {
	return &ReaderPos{
		absoluteCharPos: r.bufPos,
		relativeCharPos: r.relativePosition(),
		linePos:         r.linePos,
	}
}

// PushState pushes the current buffer state on the stack
func (r *Reader) PushState() {
	r.bufPosStack = append(r.bufPosStack, r.bufPos)
	r.linePosStack = append(r.linePosStack, r.linePos)
}

// RestoreState pops and restores the buffer position to the last pushed buffer position from the stack
func (r *Reader) RestoreState() {
	l := len(r.bufPosStack) - 1
	r.bufPos, r.bufPosStack = r.bufPosStack[l], r.bufPosStack[:l]

	l = len(r.linePosStack) - 1
	r.linePos, r.linePosStack = r.linePosStack[l], r.linePosStack[:l]
}

// PopState pops the last pushed buffer state from the stack without restoring
func (r *Reader) PopState() {
	l := len(r.bufPosStack) - 1
	r.bufPosStack = r.bufPosStack[:l]

	l = len(r.linePosStack) - 1
	r.linePosStack = r.linePosStack[:l]
}

// String gets the current buffer content between the previous pos and the current pos as string
func (r *Reader) String() string {
	prevPos := r.bufPosStack[len(r.bufPosStack)-1]
	return string(r.buf[prevPos:r.bufPos])
}

// Finished returns true if end of buffer is reached
func (r *Reader) Finished() bool {
	return r.bufPos >= r.bufPosEnd
}

// Peak returns the next rune without advancing the read position
func (r *Reader) Peak() (rn rune, err error) {
	if r.bufPos < r.bufPosEnd {
		rn = r.buf[r.bufPos]
	} else {
		err = io.EOF
	}

	return
}

// Read returns the next rune and advances the read position
func (r *Reader) Read() (rn rune, err error) {
	if r.bufPos < r.bufPosEnd {
		rn = r.buf[r.bufPos]
		r.bufPos++

		if r.bufPos < r.bufPosEnd && r.linePos < r.linePosEnd {
			if r.bufPos >= r.lines[r.linePos] {
				r.linePos++
			}
		}
	} else {
		err = io.EOF
	}

	return
}

// StringFromResult get string from match result
func (r *Reader) StringFromResult(m *MatchResult) string {
	return string(r.buf[m.BeginPos.absoluteCharPos:m.EndPos.absoluteCharPos])
}

// PushError push match result errors
func (r *Reader) PushError(failed *MatchResult) {
	r.errorStack = append(r.errorStack, failed)
}

// DeepestError returns the error that is the most advanced in char pos
func (r *Reader) DeepestError() *MatchResult {
	var deepestResult *MatchResult = nil

	for _, result := range r.errorStack {
		if deepestResult == nil {
			deepestResult = result
		} else {
			if result.EndPos.absoluteCharPos > deepestResult.EndPos.absoluteCharPos {
				deepestResult = result
			}
		}
	}

	return deepestResult
}
