package sites

import (
	"bytes"
	"testing"
)

func TestAddNormalizesAndDedups(t *testing.T) {
	s := New()
	s.Add("Example.com")
	s.Add(" example.com ")
	s.Add("")

	got := s.List()
	if len(got) != 1 || got[0] != "example.com" {
		t.Fatalf("List = %v, want [example.com]", got)
	}
}

func TestRemoveDropsItCaseInsensitively(t *testing.T) {
	s := New()
	s.Add("a.com")
	s.Add("b.com")
	s.Remove("A.COM")

	got := s.List()
	if len(got) != 1 || got[0] != "b.com" {
		t.Fatalf("List = %v, want [b.com]", got)
	}
}

func TestWriteThenReadRoundTrips(t *testing.T) {
	s := New()
	s.Add("b.com")
	s.Add("a.com")

	var buf bytes.Buffer
	if err := s.Write(&buf); err != nil {
		t.Fatal(err)
	}

	back, err := Read(&buf)
	if err != nil {
		t.Fatal(err)
	}

	got := back.List()
	if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
		t.Fatalf("round-trip = %v, want [a.com b.com]", got)
	}
}

func TestAddTakesTheDomainOutOfAPastedURL(t *testing.T) {
	s := New()
	s.Add("https://www.instagram.com/reels/abc?x=y")
	s.Add("Instagram.com")
	s.Add("youtube.com/watch?v=1")

	got := s.List()
	if len(got) != 2 || got[0] != "instagram.com" || got[1] != "youtube.com" {
		t.Fatalf("List = %v, want [instagram.com youtube.com]", got)
	}
}

func TestAddRejectsJunkWithoutADot(t *testing.T) {
	s := New()
	s.Add("notadomain")
	s.Add("   ")
	s.Add("https://www.google.com/url?sa=t&url=https://www.instagram.com/")

	got := s.List()
	if len(got) != 1 || got[0] != "google.com" {
		t.Fatalf("List = %v, want [google.com] (the pasted redirect resolves to its own host)", got)
	}
}
