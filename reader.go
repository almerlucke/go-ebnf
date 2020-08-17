package ebnf

import (
	"bufio"
	"io"
)

// Reader buffers runes to allow us to backtrack when the runes do not match a pattern
type Reader struct {
	runeBuf             []rune
	runeBufPos          int
	runeBufPosEnd       int
	runeBufPrevPosStack []int
}

// NewReader creates a new reader, all runes in input reader are first read and buffered
func NewReader(r io.Reader) (*Reader, error) {
	rr := bufio.NewReader(r)
	rs := []rune{}

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

	return &Reader{
		runeBuf:             rs,
		runeBufPosEnd:       len(rs),
		runeBufPrevPosStack: []int{0},
	}, nil
}

// PushState pushes the current buffer state on the stack
func (r *Reader) PushState() {
	r.runeBufPrevPosStack = append(r.runeBufPrevPosStack, r.runeBufPos)
}

// RestoreState pops and restores the buffer position to the last pushed buffer position from the stack
func (r *Reader) RestoreState() {
	l := len(r.runeBufPrevPosStack) - 1
	r.runeBufPos, r.runeBufPrevPosStack = r.runeBufPrevPosStack[l], r.runeBufPrevPosStack[:l]
}

// PopState pops the last pushed buffer state from the stack without restoring
func (r *Reader) PopState() {
	l := len(r.runeBufPrevPosStack) - 1
	r.runeBufPrevPosStack = r.runeBufPrevPosStack[:l]
}

// String gets the current buffer content between the previous pos and the current pos as string
func (r *Reader) String() string {
	prevPos := r.runeBufPrevPosStack[len(r.runeBufPrevPosStack)-1]
	return string(r.runeBuf[prevPos:r.runeBufPos])
}

// Finished returns true if end of buffer is reached
func (r *Reader) Finished() bool {
	return r.runeBufPos >= r.runeBufPosEnd
}

// Peak returns the next rune without advancing the read position
func (r *Reader) Peak() (rn rune, err error) {
	if r.runeBufPos < r.runeBufPosEnd {
		rn = r.runeBuf[r.runeBufPos]
	} else {
		err = io.EOF
	}

	return
}

// Read returns the next rune and advances the read position
func (r *Reader) Read() (rn rune, err error) {
	if r.runeBufPos < r.runeBufPosEnd {
		rn = r.runeBuf[r.runeBufPos]
		r.runeBufPos++
	} else {
		err = io.EOF
	}

	return
}
