package main

import (
	"crypto/rand"
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
	// Generate a random 16 byte slice 
	rand16Bytes := make([]byte, 16)
	_, err := rand.Read(rand16Bytes)
	if err != nil {
		return "", fmt.Errorf("couldn't generate random bytes for thumbnail filename: %w", err)
	}

	// Encode the random bytes to a hex string to use as the asset name
	fileName := fmt.Sprintf("%x", rand16Bytes)
	
	ext := mediaTypeToExtension(mediaType)
	return fmt.Sprintf("%s.%s", fileName, ext), nil
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func (cfg apiConfig) getObjectURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func mediaTypeToExtension(mediaType string) string {
	switch mediaType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "video/mp4":
		return "mp4"
	default:
		return "bin" // default to .bin for unknown media types
	}
}