package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/term"
)

var terminalRestoreMu sync.Mutex
var terminalRestore func()

func restoreInteractiveTerminal() {
	terminalRestoreMu.Lock()
	restore := terminalRestore
	terminalRestore = nil
	terminalRestoreMu.Unlock()
	if restore != nil {
		restore()
	}
}

// interactiveInput provides a small readline-style editor without imposing a
// heavyweight CLI framework. It falls back to ordinary lines for pipes, CI, and
// redirected input.
func interactiveInput(prompt string) <-chan string {
	output := make(chan string)
	go func() {
		defer close(output)
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
			for scanner.Scan() {
				output <- scanner.Text()
			}
			return
		}
		readRawInput(prompt, output)
	}()
	return output
}

func readRawInput(prompt string, output chan<- string) {
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	restore := func() error { return term.Restore(int(os.Stdin.Fd()), state) }
	terminalRestoreMu.Lock()
	terminalRestore = func() { _ = restore() }
	terminalRestoreMu.Unlock()
	defer func() {
		terminalRestoreMu.Lock()
		terminalRestore = nil
		terminalRestoreMu.Unlock()
		_ = restore()
	}()

	buffer := []rune{}
	cursor := 0
	history := []string{}
	historyIndex := 0
	savedDraft := ""
	previousLines := 1
	escape := []byte{}
	redraw := func() { previousLines = redrawInput(prompt, buffer, cursor, previousLines) }
	markEdited := func() {
		if historyIndex < len(history) {
			savedDraft = string(buffer)
			historyIndex = len(history)
		}
	}
	redraw()
	for {
		var bytes [8]byte
		n, readErr := os.Stdin.Read(bytes[:])
		if readErr != nil {
			if readErr == io.EOF {
				return
			}
			return
		}
		for i := 0; i < n; i++ {
			key := bytes[i]
			if len(escape) > 0 {
				escape = append(escape, key)
				if action, complete := parseEscape(escape); complete {
					escape = nil
					switch action {
					case "up":
						if len(history) > 0 {
							if historyIndex == len(history) {
								savedDraft = string(buffer)
							}
							if historyIndex > 0 {
								historyIndex--
								buffer = []rune(history[historyIndex])
								cursor = len(buffer)
								redraw()
							}
						}
					case "down":
						if historyIndex < len(history)-1 {
							historyIndex++
							buffer = []rune(history[historyIndex])
							cursor = len(buffer)
							redraw()
						} else if historyIndex == len(history)-1 {
							historyIndex = len(history)
							buffer = []rune(savedDraft)
							cursor = len(buffer)
							redraw()
						}
					case "left":
						if cursor > 0 {
							cursor--
							redraw()
						}
					case "right":
						if cursor < len(buffer) {
							cursor++
							redraw()
						}
					case "home":
						cursor = 0
						redraw()
					case "end":
						cursor = len(buffer)
						redraw()
					case "delete":
						if cursor < len(buffer) {
							markEdited()
							buffer = append(buffer[:cursor], buffer[cursor+1:]...)
							redraw()
						}
					}
				}
				continue
			}
			switch key {
			case 3: // Ctrl-C: cancel the current draft, keep the session alive.
				buffer = nil
				cursor = 0
				fmt.Print("^C\r\n")
				redraw()
			case 4: // Ctrl-D: exit only when the draft is empty.
				if len(buffer) == 0 {
					fmt.Print("\r\n")
					return
				}
			case 9: // Tab inserts spaces, avoiding terminal completion surprises.
				markEdited()
				buffer = insertRunes(buffer, cursor, []rune{' ', ' ', ' ', ' '})
				cursor += 4
				redraw()
			case 10: // Ctrl-J creates a new line; Enter submits the draft.
				markEdited()
				buffer = insertRunes(buffer, cursor, []rune{'\n'})
				cursor++
				redraw()
			case 13:
				fmt.Print("\r\n")
				text := string(buffer)
				if strings.TrimSpace(text) != "" {
					output <- text
					if len(history) == 0 || history[len(history)-1] != text {
						history = append(history, text)
					}
				}
				buffer = nil
				cursor = 0
				historyIndex = len(history)
				savedDraft = ""
				previousLines = 1
				redraw()
			case 21: // Ctrl-U clears the current draft.
				markEdited()
				buffer = nil
				cursor = 0
				redraw()
			case 23: // Ctrl-W deletes the previous word.
				markEdited()
				buffer, cursor = deletePreviousWordAtCursor(buffer, cursor)
				redraw()
			case 8, 127: // Ctrl-H / DEL: backspace.
				if cursor > 0 {
					markEdited()
					buffer = append(buffer[:cursor-1], buffer[cursor:]...)
					cursor--
					redraw()
				}
			case 1: // Ctrl-A: beginning of line.
				cursor = 0
				redraw()
			case 5: // Ctrl-E: end of line.
				cursor = len(buffer)
				redraw()
			case 27: // Start an ANSI cursor/navigation sequence.
				escape = []byte{27}
			default:
				if key < 32 || key == 127 {
					continue
				}
				if key < utf8.RuneSelf {
					markEdited()
					buffer = insertRunes(buffer, cursor, []rune{rune(key)})
					cursor++
					redraw()
					continue
				}
				// A non-ASCII byte is decoded from the bytes available in this read.
				runeValue, size := utf8.DecodeRune(bytes[i:n])
				if runeValue != utf8.RuneError || size > 1 {
					markEdited()
					buffer = insertRunes(buffer, cursor, []rune{runeValue})
					cursor++
					i += size - 1
					redraw()
				}
			}
		}
	}
}

func redrawInput(prompt string, buffer []rune, cursor, previousLines int) int {
	if previousLines > 1 {
		fmt.Printf("\x1b[%dA", previousLines-1)
	}
	lines := strings.Split(string(buffer), "\n")
	for i, line := range lines {
		fmt.Print("\r\x1b[2K")
		if i == 0 {
			fmt.Print(prompt)
		} else {
			fmt.Print("... ")
		}
		fmt.Print(line)
		if i < len(lines)-1 {
			fmt.Print("\r\n")
		}
	}
	moveCursorToPosition(prompt, string(buffer), cursor)
	return len(lines)
}

func insertRunes(buffer []rune, cursor int, value []rune) []rune {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(buffer) {
		cursor = len(buffer)
	}
	result := make([]rune, 0, len(buffer)+len(value))
	result = append(result, buffer[:cursor]...)
	result = append(result, value...)
	result = append(result, buffer[cursor:]...)
	return result
}

func parseEscape(sequence []byte) (string, bool) {
	if len(sequence) == 1 {
		return "", false
	}
	if len(sequence) == 2 && sequence[1] != '[' && sequence[1] != 'O' {
		return "", true
	}
	if len(sequence) == 2 {
		return "", false
	}
	final := sequence[len(sequence)-1]
	if final >= '0' && final <= '9' {
		return "", false
	}
	if final == '~' {
		switch string(sequence) {
		case "\x1b[1~", "\x1b[7~":
			return "home", true
		case "\x1b[4~", "\x1b[8~":
			return "end", true
		case "\x1b[3~":
			return "delete", true
		}
		return "", true
	}
	switch final {
	case 'A':
		return "up", true
	case 'B':
		return "down", true
	case 'C':
		return "right", true
	case 'D':
		return "left", true
	case 'H':
		return "home", true
	case 'F':
		return "end", true
	}
	return "", false
}

func moveCursorToPosition(prompt, text string, cursor int) {
	lines := strings.Split(text, "\n")
	cursorLine := 0
	cursorColumn := 0
	for i, line := range lines {
		if cursor <= len([]rune(line)) {
			cursorLine = i
			cursorColumn = len([]rune(line[:runeByteOffset(line, cursor)]))
			break
		}
		cursor -= len([]rune(line)) + 1
	}
	endLine := len(lines) - 1
	endColumn := len([]rune(lines[endLine]))
	if delta := endLine - cursorLine; delta > 0 {
		fmt.Printf("\x1b[%dA", delta)
	}
	endPrefix := "... "
	cursorPrefix := "... "
	if endLine == 0 {
		endPrefix = prompt
	}
	if cursorLine == 0 {
		cursorPrefix = prompt
	}
	left := visibleWidth(endPrefix) + endColumn - (visibleWidth(cursorPrefix) + cursorColumn)
	if left > 0 {
		fmt.Printf("\x1b[%dD", left)
	}
}

func runeByteOffset(text string, runeIndex int) int {
	if runeIndex <= 0 {
		return 0
	}
	count := 0
	for index := range text {
		if count == runeIndex {
			return index
		}
		count++
	}
	return len(text)
}

func visibleWidth(text string) int {
	width := 0
	inEscape := false
	for i := 0; i < len(text); i++ {
		if text[i] == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if (text[i] >= 'A' && text[i] <= 'Z') || (text[i] >= 'a' && text[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		width++
	}
	return width
}

func deletePreviousWord(buffer []rune) []rune {
	for len(buffer) > 0 && buffer[len(buffer)-1] == ' ' {
		buffer = buffer[:len(buffer)-1]
	}
	for len(buffer) > 0 && buffer[len(buffer)-1] != ' ' && buffer[len(buffer)-1] != '\n' {
		buffer = buffer[:len(buffer)-1]
	}
	return buffer
}

func deletePreviousWordAtCursor(buffer []rune, cursor int) ([]rune, int) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(buffer) {
		cursor = len(buffer)
	}
	before := deletePreviousWord(buffer[:cursor])
	after := buffer[cursor:]
	if len(before) > 0 && before[len(before)-1] == ' ' && len(after) > 0 && after[0] == ' ' {
		after = after[1:]
	}
	return append(before, after...), len(before)
}
