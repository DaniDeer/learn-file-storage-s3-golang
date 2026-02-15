package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// Parsing multipart/form-data request
	const maxMemory = 10 << 20 // Byte-shift number 10 to the left by 20 places (10 MB)
	r.ParseMultipartForm(maxMemory)

	// Get the file data from the form
	data, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get file from form", err)
		return
	}
	defer data.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", nil)
		return
	}

	// Get video metadata from DB and check if user has permission to upload thumbnail for this video
	vidMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video metadata", err)
		return
	}

	if vidMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permission to upload a thumbnail for this video", nil)
		return
	}

	mimeType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type for thumbnail", err)
		return
	}

	switch mimeType {
	case "image/jpeg", "image/png", "image/gif":
		// valid media type for thumbnail
	default:
		respondWithError(w, http.StatusBadRequest, "Unsupported media type for thumbnail. Supported types are image/jpeg, image/png, and image/gif.", nil)
		return
	}

	// Determine the file extension based on the media type and construct the asset path
	assetPath, err := getAssetPath(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate asset path for thumbnail", err)
		return
	}
	assetDiskPath := cfg.getAssetDiskPath(assetPath)
	
	// Save the file to the server's filesystem
	file, err := os.Create(assetDiskPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file on server", err)
		return
	}
	defer file.Close()

	if _, err := io.Copy(file, data); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save thumbnail file on server", err)
		return
	}

	// Create a public URL for the fileserver for the thumbnail and update the video metadata in the database
	thumbnailURL := cfg.getAssetURL(assetPath)
	
	vidMetadata.ThumbnailURL = &thumbnailURL

	// Update the video metadata in the database with the thumbnail URL
	if err := cfg.db.UpdateVideo(vidMetadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video metadata with thumbnail URL", err)
		return
	}

	// Respond with the updated video metadata
	respondWithJSON(w, http.StatusOK, vidMetadata)

}
