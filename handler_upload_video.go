package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// Set upload size limit to 1 GB
	const uploadLimit = 1 << 30 // Byte-shift number 1 to the left by 30 places (1 GB)
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)
	
	// Parse vidoID as a UUID from the URL path
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// Get the JWT from the Authorization header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	// Validate the JWT and get the userID
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	// Get video metadata from DB and check if user has permission to upload video
	vidMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video metadata", err)
		return
	}

	if vidMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't have permission to upload a thumbnail for this video", nil)
		return
	}

	// Get the file data from the form
	data, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get file from form", err)
		return
	}
	defer data.Close()

	// Check if Content-Type header is present
	mediaType := fileHeader.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type", nil)
		return
	}

	// Parse the media type from the Content-Type header and check if it is "video/mp4"
	mimeType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}

	switch mimeType {
	case "video/mp4":
		// valid media type for video upload
	default:
		respondWithError(w, http.StatusBadRequest, "Unsupported media type. Only video/mp4 is allowed", nil)
		return
	}

	// Save the uploaded video to disk temporarely before uploading to S3
	file, err := os.CreateTemp("", fmt.Sprintf("tubely-upload-*.%s", mediaTypeToExtension(mediaType)))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create video file on server", err)
		return
	}
	defer os.Remove(file.Name()) // clean up the temp file after we're done
	defer file.Close()  // Close before os.Remove to ensure the file is not in use when we try to delete it

	// Copy the uploaded video data from the wire to the temp file
	if _, err = io.Copy(file, data); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save video file on server", err)
		return
	}

	// Upload the video file to S3

	// Reset the file pointer to the beginning of the file before uploading to S3
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't reset file pointer for video upload", err)
		return
	}

	// Generate key for the S3 object using random-32-byte-hex string
	key, err := getAssetPath(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate S3 object key", err)
		return
	}

	// Create the S3 PutObjectInput with the video file as the Body
	input := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),      // env variable for S3 bucket name
		Key:         aws.String(key),               // use the generated key as the S3 object key
		Body:        file,    							 				// the video file to upload
		ContentType: aws.String(mediaType),         // set the Content-Type metadata for the S3 object
	}

	_, err = cfg.s3Client.PutObject(r.Context(), input)  // Use the request context for cancellation and timeouts
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video to S3", err)
		return
	}

	// Update the video record in the database with the S3 key and URL
	s3URL := cfg.getObjectURL(key)
	vidMetadata.VideoURL = &s3URL

	if err := cfg.db.UpdateVideo(vidMetadata); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video metadata with thumbnail URL", err)
		return
	}

	// Respond with the updated video metadata
	respondWithJSON(w, http.StatusOK, vidMetadata)

}

