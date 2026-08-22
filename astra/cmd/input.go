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
	previousLines := 1
	lastInputWasPaste := false
	redraw := func() { previousLines = redrawInput(prompt, buffer, previousLines) }
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
			switch key {
			case 3: // Ctrl-C: cancel the current draft, keep the session alive.
				buffer = nil
				fmt.Print("^C\r\n")
				redraw()
				lastInputWasPaste = false
			case 4: // Ctrl-D: exit only when the draft is empty.
				if len(buffer) == 0 {
					fmt.Print("\r\n")
					return
				}
			case 9: // Tab inserts spaces, avoiding terminal completion surprises.
				buffer = append(buffer, ' ', ' ', ' ', ' ')
				redraw()
			case 10: // Ctrl-J creates a new line; Enter submits the draft.
				buffer = append(buffer, '\n')
				redraw()
				lastInputWasPaste = true
			case 13:
				fmt.Print("\r\n")
				text := string(buffer)
				if strings.TrimSpace(text) != "" {
					output <- text
				}
				buffer = nil
				previousLines = 1
				lastInputWasPaste = false
				redraw()
			case 21: // Ctrl-U clears the current draft.
				buffer = nil
				redraw()
			case 23: // Ctrl-W deletes the previous word.
				buffer = deletePreviousWord(buffer)
				redraw()
			case 27: // Consume common escape sequences so arrow keys do not become text.
				if i+2 < n && bytes[i+1] == '[' {
					i += 2
				}
			default:
				if key < 32 || key == 127 {
					continue
				}
				if key < utf8.RuneSelf {
					buffer = append(buffer, rune(key))
					redraw()
					continue
				}
				// A non-ASCII byte is decoded from the bytes available in this read.
				runeValue, size := utf8.DecodeRune(bytes[i:n])
				if runeValue != utf8.RuneError || size > 1 {
					buffer = append(buffer, runeValue)
					i += size - 1
					redraw()
				}
			}
		}
		_ = lastInputWasPaste // reserved for bracketed-paste support.
	}
}

func redrawInput(prompt string, buffer []rune, previousLines int) int {
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
	return len(lines)
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
