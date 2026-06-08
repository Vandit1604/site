package handlers

import (
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// galleryDir is the on-disk folder scanned for personal photos.
// Drop image files in here and they show up automatically.
const galleryDir = "static/images/gallery"

// galleryURLBase is the public URL prefix the files are served from.
const galleryURLBase = "/static/images/gallery"

// imageExtensions are the file types treated as gallery photos.
var imageExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".avif": true,
	".gif":  true,
}

// ReadGalleryPhotos returns the public URLs of every image in the gallery
// folder, sorted by filename. Missing folder or read errors yield an empty
// slice so the homepage degrades gracefully instead of failing.
func ReadGalleryPhotos() []string {
	entries, err := os.ReadDir(galleryDir)
	if err != nil {
		return nil
	}

	photos := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		ext := strings.ToLower(path.Ext(name))
		if !imageExtensions[ext] {
			continue
		}
		photos = append(photos, galleryURLBase+"/"+name)
	}

	sort.Strings(photos)
	return photos
}

// ShowGalleryPage renders the dedicated photo gallery page.
func ShowGalleryPage(c *gin.Context) {
	c.HTML(http.StatusOK, "gallery.html", gin.H{
		"galleryPhotos": ReadGalleryPhotos(),
	})
}
