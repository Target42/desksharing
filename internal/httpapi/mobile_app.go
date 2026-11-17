package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const androidAPKName = "desksharing.apk"

// AndroidAppHandlers serves the published Flutter APK from DATA_DIR/mobile.
// The download is public so the login page can offer it before a session exists.
type AndroidAppHandlers struct {
	DataDir string
}

func (h AndroidAppHandlers) Info(w http.ResponseWriter, r *http.Request) {
	_, info, ok := findAndroidAPK(h.DataDir)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"available": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"available": true,
		"url":       "/download/" + androidAPKName,
		"filename":  androidAPKName,
		"bytes":     info.Size(),
	})
}

func (h AndroidAppHandlers) Download(w http.ResponseWriter, r *http.Request) {
	path, info, ok := findAndroidAPK(h.DataDir)
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Disposition", `attachment; filename="`+androidAPKName+`"`)
	http.ServeContent(w, r, androidAPKName, info.ModTime(), f)
}

// findAndroidAPK returns desksharing.apk, or the only other *.apk in the
// mobile directory. Symlinks and nested paths are ignored.
func findAndroidAPK(dataDir string) (string, os.FileInfo, bool) {
	root := strings.TrimSpace(dataDir)
	if root == "" {
		return "", nil, false
	}
	dir := filepath.Join(root, "mobile")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, false
	}
	var only string
	var count int
	var preferred string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !strings.EqualFold(filepath.Ext(name), ".apk") {
			continue
		}
		if strings.ContainsAny(name, `/\`) {
			continue
		}
		count++
		only = name
		if strings.EqualFold(name, androidAPKName) {
			preferred = name
		}
	}
	name := preferred
	if name == "" && count == 1 {
		name = only
	}
	if name == "" {
		return "", nil, false
	}
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", nil, false
	}
	return path, info, true
}
