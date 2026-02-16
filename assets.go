package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string, prefix ...string) (string, error) {
	// Generate a random 16 byte slice 
	rand16Bytes := make([]byte, 16)
	_, err := rand.Read(rand16Bytes)
	if err != nil {
		return "", fmt.Errorf("couldn't generate random bytes for thumbnail filename: %w", err)
	}

	// Encode the random bytes to a hex string to use as the asset name
	fileName := fmt.Sprintf("%x", rand16Bytes)
	
	ext := mediaTypeToExtension(mediaType)
	
	if len(prefix) > 0 && prefix[0] != "" {
		fileName = fmt.Sprintf("%s/%s", prefix[0], fileName)
	}
	
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

// getVideoAspectRatio uses ffprobe to determine the aspect ratio of a video file and categorizes it as "16:9", "9:16", "1:1", "landscape", "portrait", or "other"
// ffprobe and ffmpeg are command-line tools that are part of the FFmpeg project, which is a powerful multimedia framework for processing video and audio files.
// These tools can be installed on your system and are used to analyze and manipulate multimedia files. In this case, we use ffprobe to extract the width and height of the video, which allows us to calculate the aspect ratio and categorize it accordingly.
func getVideoAspectRatio(filePath string) (string, error) {

	// ffprobe -v error -print_format json -show_streams ./samples/boots-video-horizontal.mp4
	output, err := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath).Output()
	if err != nil {
		return "", fmt.Errorf("ffprobe command failed: %w", err)
	}

	type ffprobeStream struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}

	type ffprobeOutput struct {
		Streams []ffprobeStream `json:"streams"`
	}

	var probeData ffprobeOutput
	if err := json.Unmarshal(output, &probeData); err != nil {
		return "", fmt.Errorf("couldn't parse ffprobe output: %w", err)
	}

	if len(probeData.Streams) == 0 {
		return "", fmt.Errorf("no streams found in ffprobe output")
	}

	width := probeData.Streams[0].Width
	height := probeData.Streams[0].Height
	aspectRatio := float64(width) / float64(height)

	// Check if the aspect ratio is approximately 16:9, 9:16, or 1:1 with a small tolerance to account for minor variations
	const tolerance = 0.01
	if math.Abs(aspectRatio-16.0/9.0) < tolerance {
		return "landscape", nil
	} else if math.Abs(aspectRatio-9.0/16.0) < tolerance {
		return "portrait", nil
	} else if math.Abs(aspectRatio-1.0) < tolerance {
		return "other", nil
	}

	// Another way to categorize aspect ratio without using floating point comparison.
	// Instead using integer math to check if the width and height are in a 16:9 or 9:16 ratio, allowing for some tolerance by checking if the width is approximately 16/9 times the height or vice versa.
	if width == 16*height/9 {
		return "landscape", nil
	} else if height == 16*width/9 {
		return "portrait", nil
	}
	return "other", nil

}


// processVidoForFastStart takes a file path as input and creates and returns a new path to a file with "fast start" encoding.
func processVidoForFastStart(filePath string) (string, error) {

	// The input file path is: /var/www/assets/abc123.mp4
	// We want the output file path to be: /var/www/assets/abc123.processing.mp4
	ext := filepath.Ext(filePath) // .mp4
	base := filePath[:len(filePath)-len(ext)] // /var/www/assets/abc123
	outputFilepath := fmt.Sprintf("%s.processing%s", base, ext) // /var/www/assets/abc123.processing.mp4

	// Use ffmpeg to process the video for fast start streaming
	_, err := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilepath).Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg command failed: %w", err)
	}

	fileInfo, err := os.Stat(outputFilepath)
	if err != nil {
		return "", fmt.Errorf("couldn't stat processed video file: %w", err)
	}

	if fileInfo.Size() == 0 {
		return "", fmt.Errorf("processed video file is empty, something went wrong with ffmpeg processing")
	}

	return outputFilepath, nil
}