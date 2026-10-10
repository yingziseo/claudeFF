package browser

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectZipTraversal(t *testing.T) {
	for _, name := range []string{"../outside", "C:/bad", "..\\outside"} {
		dir := t.TempDir()
		f, _ := os.Create(filepath.Join(dir, "a.zip"))
		w := zip.NewWriter(f)
		entry, _ := w.Create(name)
		entry.Write([]byte("bad"))
		w.Close()
		f.Close()
		if Extract(f.Name(), filepath.Join(dir, "out")) == nil {
			t.Fatal("accepted", name)
		}
	}
}
