package core

import (
	"bytes"
	"context"
	"testing"

	"9itchi-resume-api/models/api"
)

func entryContent() api.ResumeContent {
	t := false
	return api.ResumeContent{
		Contact: api.Contact{Name: "Ada Lovelace"},
		Projects: []api.Project{
			{Name: "Visible project", Tech: "Go, React", Description: []string{"Built it"}, URL: "https://example.com/p"},
			{Name: "Hidden project", Tech: "Rust", Hidden: true},
			{Name: "No tech shown", Tech: "Zig", HideTech: true},
		},
		Experience: []api.Experience{{Title: "Dev", Company: "Acme", Hidden: true}},
		Style:      &api.CVStyle{ListStyle: "hyphen", IndentBody: &t, SubtitleStyle: "italic", DateStyle: "bold"},
	}
}

func TestHiddenEntriesAndTech(t *testing.T) {
	c := visibleEntries(entryContent())
	if len(c.Projects) != 2 || len(c.Experience) != 0 {
		t.Fatalf("hidden entries kept: %d projects, %d experiences", len(c.Projects), len(c.Experience))
	}
	if techText(c.Projects[0], "en") != "Technologies: Go, React" {
		t.Fatalf("tech line: %q", techText(c.Projects[0], "en"))
	}
	if techText(c.Projects[1], "en") != "" {
		t.Fatal("hidden tech must be empty")
	}
}

func TestSafeURL(t *testing.T) {
	for in, want := range map[string]string{"https://a.b": "https://a.b", "javascript:alert(1)": "", "file:///etc/passwd": "", " http://x.y ": "http://x.y"} {
		if got := safeURL(in); got != want {
			t.Errorf("safeURL(%q)=%q want %q", in, got, want)
		}
	}
}

func TestEntryStylesRender(t *testing.T) {
	for _, layout := range []string{"classic", "modern", "compact"} {
		for _, e := range []Exporter{NewPDFExporter(), NewDOCXExporter()} {
			b, err := e.Render(context.Background(), entryContent(), layout, RenderOptions{Language: "en"})
			if err != nil || len(b) == 0 {
				t.Fatalf("%s %s: %v", layout, e.Format(), err)
			}
			if e.Format() == "pdf" && !bytes.HasPrefix(b, []byte("%PDF")) {
				t.Fatalf("%s: not a pdf", layout)
			}
		}
	}
}
