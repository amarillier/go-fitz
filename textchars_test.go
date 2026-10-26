//go:build cgo && !nocgo

package fitz_test

import (
	"path/filepath"
	"testing"

	"github.com/gen2brain/go-fitz"
)

// TextChars' runes must concatenate to exactly Text's string (that's the
// contract callers rely on to map a match in Text to character boxes), and
// real characters must have sane, ordered quads.
func TestTextChars_MatchesTextAndHasQuads(t *testing.T) {
	doc, err := fitz.New(filepath.Join("testdata", "test.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()

	for n := 0; n < doc.NumPage(); n++ {
		want, err := doc.Text(n)
		if err != nil {
			t.Fatal(err)
		}
		chars, err := doc.TextChars(n)
		if err != nil {
			t.Fatal(err)
		}
		var got []rune
		real := 0
		for _, c := range chars {
			got = append(got, c.R)
			if c.R == '\n' {
				continue
			}
			real++
			// upper-left is above-left of lower-right (y grows downward)
			if !(c.Quad[0] <= c.Quad[6]+0.01 && c.Quad[1] <= c.Quad[7]+0.01) {
				t.Errorf("page %d char %q has a malformed quad %v", n, c.R, c.Quad)
			}
			if c.Size <= 0 {
				t.Errorf("page %d char %q has size %v", n, c.R, c.Size)
			}
		}
		if string(got) != want {
			t.Errorf("page %d: TextChars runes != Text\n got %q\nwant %q", n, string(got), want)
		}
		if want != "" && real == 0 {
			t.Errorf("page %d has text but no characters", n)
		}
	}
	if _, err := doc.TextChars(doc.NumPage()); err == nil {
		t.Error("expected an error past the last page")
	}
}
