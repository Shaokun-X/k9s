package view

import (
	"bytes"
	"regexp"

	"github.com/derailed/tcell/v2"
	"github.com/derailed/tview"
)

// FastTextView is a basic, line wrapped text view that is designed to replicate a
// severely cut down version of tview's TextView, removing color support, grapheme cluster
// handling, regions and other functionality with the aim of increasing performance
// and being able to handle megabytes of data with wrapping.
// You probably don't want to use this.
// See: https://github.com/rivo/tview/issues/686
type FastTextView struct {
	*tview.TextView

	buffer     []string
	lineOffset int  // the line offset for view scrolling
	fitsAll    bool // whether or not the entire content of buffer from the lineOffset onwards fits on the screen
	wrap       bool
}

var (
	newLineRegex = regexp.MustCompile(`\r?\n`)
	TabSize      = 4
)

func NewFastTextView() *FastTextView {
	var buffer []string

	return &FastTextView{
		TextView:   tview.NewTextView(),
		buffer:     buffer,
		lineOffset: 0,
		wrap:       true,
	}
}

func (t *FastTextView) SetWrap(wrap bool) *FastTextView {
	t.wrap = wrap
	return t
}

// Write lets us implement the io.Writer interface. Tab characters will be
// replaced with TabSize space characters. A "\n" or "\r\n" will be interpreted
// as a new line.
func (t *FastTextView) Write(p []byte) (n int, err error) {
	t.Lock()
	defer t.Unlock()

	newBytes := bytes.Replace(p, []byte{'\t'}, bytes.Repeat([]byte{' '}, TabSize), -1)
	for index, line := range newLineRegex.Split(string(newBytes), -1) {
		if index == 0 {
			if len(t.buffer) == 0 {
				t.buffer = []string{line}
			} else {
				t.buffer[len(t.buffer)-1] += line
			}
		} else {
			t.buffer = append(t.buffer, line)
		}
	}

	return len(p), nil
}

func (t *FastTextView) Clear() {
	t.Lock()
	defer t.Unlock()
	t.buffer = nil
	t.lineOffset = 0
}

func (t *FastTextView) ScrollToBeginning() {
	t.Lock()
	defer t.Unlock()
	t.lineOffset = 0
}

func (t *FastTextView) Draw(screen tcell.Screen) {
	t.Lock()
	defer t.Unlock()

	t.Box.DrawForSubclass(screen, t)
	t.fitsAll = true
	x, y, width, height := t.GetInnerRect()

	// loop each str and print
	index, offsetindex := 0, 0

	// wrap implementation
	if t.wrap {
		return
	}

	for _, str := range t.buffer {
		if index >= height {
			t.fitsAll = false
			break
		}

		if len(str) == 0 { // blank line
			if offsetindex < t.lineOffset {
				offsetindex++
			} else {
				index++
			}
		}

		runes := []rune(str)
		for len(runes) > 0 {
			var extract []rune
			if index >= height {
				t.fitsAll = false
				break
			}

			if len(runes) > width {
				extract = runes[:width]
			} else {
				extract = runes
			}

			w := len(extract)
			for len(string(extract)) > width {
				w-- // string width is greater than rune count, yank one out
				extract = runes[:w]
			}

			runes = runes[len(extract):]
			if offsetindex < t.lineOffset {
				offsetindex++
				continue
			} else {
				tview.PrintSimple(screen, string(extract), x, y+index)
				index++
			}
		}
	}
}

func (t *FastTextView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return t.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {

		_, _, width, height := t.GetInnerRect()
		switch event.Key() {

		case tcell.KeyRune:
			switch event.Rune() {
			case 'g': // back to the beginning
				t.lineOffset = 0
			case 'G': // end
				end := 0
				for _, str := range t.buffer {
					length := len(str)
					end = end + (length / width)
					if length%width > 0 {
						end++
					}
				}
				t.lineOffset = end - height
			case 'j':
				if !t.fitsAll {
					max := 0
					for _, str := range t.buffer {
						length := len(str)
						max = max + (length / width)
						if length%width > 0 {
							max++
						}
						if t.lineOffset < max {
							t.lineOffset++
							break
						}
					}
				}
			case 'k':
				if t.lineOffset > 0 {
					t.lineOffset--
				}
			}

		case tcell.KeyHome:
			t.lineOffset = 0 // back to the beginning
		case tcell.KeyEnd:
			end := 0
			for _, str := range t.buffer {
				length := len(str)
				end = end + (length / width)
				if length%width > 0 {
					end++
				}
			}
			t.lineOffset = end - height
		case tcell.KeyUp:
			if t.lineOffset > 0 {
				t.lineOffset--
			}
		case tcell.KeyDown:
			if !t.fitsAll {
				max := 0
				for _, str := range t.buffer {
					length := len(str)
					max = max + (length / width)
					if length%width > 0 {
						max++
					}
					if t.lineOffset < max {
						t.lineOffset++
						break
					}
				}
			}
		case tcell.KeyPgUp, tcell.KeyCtrlB:
			if t.lineOffset > 0 {
				t.lineOffset = t.lineOffset - height
				if t.lineOffset < 0 {
					t.lineOffset = 0
				}
			}

		case tcell.KeyPgDn, tcell.KeyCtrlF:
			if !t.fitsAll {
				max := 0
				for _, str := range t.buffer {
					length := len(str)
					max = max + (length / width)
					if length%width > 0 {
						max++
					}
					if t.lineOffset < max {
						t.lineOffset = t.lineOffset + height
						break
					}
				}
			}
		}

	})
}

func (t *FastTextView) MouseHandler() func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
	return t.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(p tview.Primitive)) (consumed bool, capture tview.Primitive) {
		x, y := event.Position()
		if !t.InRect(x, y) {
			return false, nil
		}

		_, _, width, _ := t.GetInnerRect()

		switch action {
		case tview.MouseLeftClick:
			setFocus(t)
			consumed = true
		case tview.MouseScrollUp:
			if t.lineOffset > 0 {
				t.lineOffset--
			}
			consumed = true
		case tview.MouseScrollDown:
			if !t.fitsAll {
				max := 0
				for _, str := range t.buffer {
					length := len(str)
					max = max + (length / width)
					if length%width > 0 {
						max++
					}
					if t.lineOffset < max {
						t.lineOffset++
						break
					}
				}
			}
			consumed = true
		}

		return
	})
}
