package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) (string, error) {
	// Generate a random 32 byte slice 
	rand32Bytes := make([]byte, 32)
	_, err := rand.Read(rand32Bytes)
	if err != nil {
		return "", fmt.Errorf("couldn't generate random bytes for thumbnail filename: %w", err)
	}

	// Encode the random bytes to a base64 string to use as the asset name
	fileName := base64.RawURLEncoding.EncodeToString(rand32Bytes)
	
	ext := mediaTypeToExtension(mediaType)
	return fmt.Sprintf("%s.%s", fileName, ext), nil
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExtension(mediaType string) string {
	switch mediaType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	default:
		return "bin" // default to .bin for unknown media types
	}
}