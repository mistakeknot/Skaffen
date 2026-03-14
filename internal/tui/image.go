package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mistakeknot/Skaffen/internal/provider"
)

const maxImageSize = 5 * 1024 * 1024 // 5MB — Anthropic limit

var imageExts = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
}

// isImageFile returns true if the path has a recognized image extension.
func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return imageExts[ext]
}

// mediaTypeFromExt returns the MIME type for an image extension.
func mediaTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

// imageBadge returns a TUI-friendly placeholder for an image.
func imageBadge(filename string, sizeBytes int64) string {
	return fmt.Sprintf("[img %s (%s)]", filename, formatSize(sizeBytes))
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%dKB", bytes/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// ExpandImageMentions extracts image @references from text and returns
// image ContentBlocks plus the cleaned text (with badges replacing refs).
// Non-image @mentions are left untouched for expandAtMentions to handle.
func ExpandImageMentions(text, workDir string) (string, []provider.ContentBlock) {
	if !strings.Contains(text, "@") {
		return text, nil
	}

	var blocks []provider.ContentBlock

	cleaned := atMentionRe.ReplaceAllStringFunc(text, func(match string) string {
		path := match[1:] // strip @
		if !isImageFile(path) {
			return match // not an image, leave for expandAtMentions
		}

		fullPath := path
		if !filepath.IsAbs(path) && workDir != "" {
			fullPath = filepath.Join(workDir, path)
		}

		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			return match // file not found
		}
		if info.Size() > maxImageSize {
			return match // too large, leave as-is
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			return match
		}

		ext := filepath.Ext(path)
		blocks = append(blocks, provider.ContentBlock{
			Type: "image",
			Source: &provider.ImageSource{
				Type:      "base64",
				MediaType: mediaTypeFromExt(ext),
				Data:      base64.StdEncoding.EncodeToString(data),
			},
		})

		return imageBadge(filepath.Base(path), info.Size())
	})

	return cleaned, blocks
}
