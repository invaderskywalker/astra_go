package main

import "testing"

func TestParseEscapeNavigation(t *testing.T) {
	cases := map[string]string{
		"\x1b[A":    "up",
		"\x1b[B":    "down",
		"\x1b[C":    "right",
		"\x1b[D":    "left",
		"\x1b[H":    "home",
		"\x1b[F":    "end",
		"\x1b[3~":   "delete",
		"\x1b[200~": "paste_start",
		"\x1b[201~": "paste_end",
		"\x1b[1~":   "home",
		"\x1b[4~":   "end",
		"\x1bOH":    "home",
		"\x1bOF":    "end",
	}
	for sequence, expected := range cases {
		actual, complete := parseEscape([]byte(sequence))
		if !complete || actual != expected {
			t.Fatalf("parseEscape(%q) = %q, %v; want %q, true", sequence, actual, complete, expected)
		}
	}
}

func TestInsertRunesAtCursor(t *testing.T) {
	got := string(insertRunes([]rune("ac"), 1, []rune("b")))
	if got != "abc" {
		t.Fatalf("insertRunes returned %q, want abc", got)
	}
}

func TestDeletePreviousWordRespectsCursor(t *testing.T) {
	buffer := []rune("one two three")
	updated, _ := deletePreviousWordAtCursor(buffer, 7)
	got := string(updated)
	if got != "one three" {
		t.Fatalf("word deletion returned %q, want %q", got, "one three")
	}
}
