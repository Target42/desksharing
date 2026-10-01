package catalog

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectImageExt(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	if ext, ok := detectImageExt(png); !ok || ext != ".png" {
		t.Fatalf("png %s %v", ext, ok)
	}
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0}
	if ext, ok := detectImageExt(jpeg); !ok || ext != ".jpg" {
		t.Fatalf("jpeg %s %v", ext, ok)
	}
	webp := append([]byte("RIFF"), 0, 0, 0, 0)
	webp = append(webp, []byte("WEBP")...)
	if ext, ok := detectImageExt(webp); !ok || ext != ".webp" {
		t.Fatalf("webp %s %v", ext, ok)
	}
	html := []byte("<html><script>alert(1)</script></html>")
	if _, ok := detectImageExt(html); ok {
		t.Fatal("html must not pass as an image")
	}
}

func TestPathInsideRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	abs, err := pathInside(root, "floor-plans/tenant/plan.png")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("abs %s root %s", abs, root)
	}
	if _, err := pathInside(root, "../secret"); err == nil {
		t.Fatal("expected traversal to fail")
	}
	if _, err := pathInside(root, filepath.Join("floor-plans", "..", "..", "secret")); err == nil {
		t.Fatal("expected nested traversal to fail")
	}
}

func TestImageContentType(t *testing.T) {
	if imageContentType("floor-plans/a/b.png") != "image/png" {
		t.Fatal("png")
	}
	if imageContentType("floor-plans/a/b.html") != "" {
		t.Fatal("html")
	}
}
