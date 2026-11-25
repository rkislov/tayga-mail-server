package mmplugins

import (
	"net/http"
	"os"
	"path/filepath"
)

// Only the active manifest's JavaScript and the community host entry are served.
// No arbitrary bundle path, source directory, private SQL binding or data file
// can be selected by the request.
func (r *Runtime) ServeProductAsset(w http.ResponseWriter, req *http.Request, id, name string) {
	if req.Method != "GET" && req.Method != "HEAD" {
		w.WriteHeader(405)
		return
	}
	var relative string
	for _, bundle := range r.env.Active() {
		if bundle.Manifest.Id != id || !bundle.Manifest.HasWebapp() {
			continue
		}
		if name == "main.js" {
			relative = bundle.Manifest.Webapp.BundlePath
		}
		if id == "playbooks" && name == "tms-host.js" {
			relative = "webapp/dist/tms-host.js"
		}
	}
	if relative == "" {
		http.NotFound(w, req)
		return
	}
	root, err := os.OpenRoot(filepath.Join(r.options.Directory, id))
	if err != nil {
		http.NotFound(w, req)
		return
	}
	defer root.Close()
	file, err := root.Open(relative)
	if err != nil {
		http.NotFound(w, req)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 20<<20 {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, req, info.Name(), info.ModTime(), file)
}
