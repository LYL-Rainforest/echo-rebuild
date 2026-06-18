package engine

import (
	"context"
	"strings"
	"testing"
)

func TestParseSearch_Empty(t *testing.T) {
	u, err := parseSearchResults(strings.NewReader("<html></html>"))
	if err != nil { t.Fatal(err) }
	if len(u) != 0 { t.Fatalf("got %d", len(u)) }
}

func TestParseSearch_Basic(t *testing.T) {
	h := `<html><a href="https://ex.com/dl">D</a><a href="https://ex.com/pkg">P</a></html>`
	u, _ := parseSearchResults(strings.NewReader(h))
	if len(u) != 2 || u[0] != "https://ex.com/dl" { t.Fatalf("got %v", u) }
}

func TestParseSearch_Dedup(t *testing.T) {
	h := `<a href="https://x.com">A</a><a href="https://x.com">B</a>`
	u, _ := parseSearchResults(strings.NewReader(h))
	if len(u) != 1 { t.Fatal("dedup failed") }
}

func TestParseSearch_FilterAds(t *testing.T) {
	h := `<a href="https://good.com">G</a><a href="https://www.googleadservices.com/ad">Ad</a><a href="https://duckduckgo.com/redirect">DDG</a>`
	u, _ := parseSearchResults(strings.NewReader(h))
	if len(u) != 1 || u[0] != "https://good.com" { t.Fatalf("filter failed: %v", u) }
}

func TestParseSearch_Max5(t *testing.T) {
	var b strings.Builder
	b.WriteString("<html>")
	for i := 0; i < 10; i++ { b.WriteString("<a href=\"https://x.com/"); b.WriteByte(byte('a'+i)); b.WriteString("\">X</a>") }
	b.WriteString("</html>")
	u, _ := parseSearchResults(strings.NewReader(b.String()))
	if len(u) > 5 { t.Fatalf("got %d, max 5", len(u)) }
}

func TestParseSearch_RelativeIgnored(t *testing.T) {
	h := `<a href="/rel">R</a><a href="https://abs.com">A</a>`
	u, _ := parseSearchResults(strings.NewReader(h))
	if len(u) != 1 { t.Fatal("relative ignored") }
}

func TestParseSearch_Malformed(t *testing.T) {
	for _, h := range []string{"", `<a href="https://x.com`, `<a href="https://x.com">X`} {
		u, err := parseSearchResults(strings.NewReader(h))
		if err != nil { t.Fatal(err) }
		_ = u
	}
}

func TestAutoSearchURL_CancelledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewInstaller("").AutoSearchURL(ctx, "firefox")
	if err == nil { t.Fatal("expected error for cancelled ctx") }
}

func TestNewInstaller(t *testing.T) {
	if NewInstaller("/tmp/dl").DownloadDir != "/tmp/dl" { t.Fatal("dir not set") }
}

func TestNewInstaller_EmptyDir(t *testing.T) {
	if NewInstaller("") == nil { t.Fatal("nil installer") }
}
