package main

import (
	"io"
	"os"
	"fmt"
	"mime"
	"net/http"
	"encoding/base64"
	"crypto/rand"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/aws"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxVideoSize = 1 << 30 // 1GB
	http.MaxBytesReader(w, r.Body, maxVideoSize)

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

	fmt.Println("uploading video to s3", videoID, "by user", userID)

	vide_metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to retreive video metadata from database", err)
		return
	}
	if vide_metadata.CreateVideoParams.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "", err)
		return
	}

	r.ParseMultipartForm(maxVideoSize)
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	media_type := header.Header.Get("Content-Type")
	m_type, _, err := mime.ParseMediaType(media_type)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video file provided", err)
		return
	}
	if m_type != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Not mp4 file, only mp4 files are supported", err)
		return
	}

	temp_file_ptr, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed with temporary file creation opration", err)
		return
	}
	defer os.Remove(temp_file_ptr.Name())
	defer temp_file_ptr.Close()
	reader := io.Reader(file)
	writer := io.Writer(temp_file_ptr)
	_, err = io.Copy(writer, reader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to save video as temporary file", err)
		return
	}
	temp_file_ptr.Seek(0, io.SeekStart)

	processed_file_path, err := processVideoForFastStart(temp_file_ptr.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to process the file for fast start", err)
		return
	}
	defer os.Remove(processed_file_path)
	processed_file, _ := os.Open(processed_file_path)
	defer processed_file.Close()
	pf_reader := io.Reader(processed_file)

	ratio, err := getVideoAspectRatio(temp_file_ptr.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to extract aspect ratio data from video file", err)
		return
	}
	var dir_str string
	switch ratio {
		case "16:9":
			dir_str = "landscape"
		case "9:16":
			dir_str = "portrait"
		default:
			dir_str = "other"
	}

	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to generate random filename", err)
		return
	}
	name := base64.RawURLEncoding.EncodeToString(key)
	file_name := fmt.Sprintf("%s/%s.mp4",dir_str, name)

	object_intput := s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(file_name),
		Body:        pf_reader,
		ContentType: aws.String(media_type),
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &object_intput)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to store file in S3", err)
		return
	}

	video_url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, file_name)
	vide_metadata.VideoURL = &video_url

	err = cfg.db.UpdateVideo(vide_metadata)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update video metadata in database", err)
		return
	}
}
