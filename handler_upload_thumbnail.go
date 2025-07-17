package main

import (
	"io"
	"fmt"
	"net/http"

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

	const maxMemory = 10 << 20 // 10 MB using bit shifting
	r.ParseMultipartForm(maxMemory)

	file, _, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	media_type := r.Header.Get("Content-Type")

	byte_data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse the file to byte data", err)
		return
	}

	vide_metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to retreive video metadata from database", err)
		return
	}
	if vide_metadata.CreateVideoParams.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "", err)
		return
	}

	tn := thumbnail{
		data:      byte_data,
		mediaType: media_type,
	}
	videoThumbnails[videoID] = tn

	tn_url := fmt.Sprintf("http://localhost:%s/api/thumbnails/%v", cfg.port, videoID)

	vide_metadata.ThumbnailURL = &tn_url

	err = cfg.db.UpdateVideo(vide_metadata)
	if err != nil {
		delete(videoThumbnails, videoID)
		respondWithError(w, http.StatusBadRequest, "Unable to update metadata in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, vide_metadata)
}
