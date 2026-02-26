package main

import (
	"game-engine/internal/download"
)

// downloadImage fetches the image at url and saves it under dir (e.g. "assets/textures/downloaded").
// Returns the relative path to the saved file (e.g. "assets/textures/downloaded/abc.png") and an error.
func downloadImage(url string, dir string) (relPath string, err error) {
	return download.Download(url, dir)
}
