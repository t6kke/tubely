package main

import (
	"os"
	"io"
	"fmt"
	"mime"
	"strings"
	"net/http"
	"encoding/base64"
	"crypto/rand"
	"path/filepath"

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

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	//media_type := r.Header.Get("Content-Type")
	media_type := header.Header.Get("Content-Type")

	// part of v1
	/*byte_data, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse the file to byte data", err)
		return
	}*/

	vide_metadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to retreive video metadata from database", err)
		return
	}
	if vide_metadata.CreateVideoParams.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "", err)
		return
	}

	// part of v1
	/*tn := thumbnail{
		data:      byte_data,
		mediaType: media_type,
	}
	videoThumbnails[videoID] = tn*/

	// v1 storing in memory - old
	//tn_url := fmt.Sprintf("http://localhost:%s/api/thumbnails/%v", cfg.port, videoID)
	//vide_metadata.ThumbnailURL = &tn_url

	// v2 storing in db as base64 encoded data - old
	//base_64_data := base64.StdEncoding.EncodeToString(byte_data)
	//tn_url := fmt.Sprintf("data:%s;base64,%s", media_type, base_64_data)
	//vide_metadata.ThumbnailURL = &tn_url

	// v3 storing in ./assets/directory
	m_type, _, err := mime.ParseMediaType(media_type)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnain filetype provided", err)
		return
	}
	if m_type != "image/jpeg" && m_type != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid thumbnain filetype provided", err)
		return
	}
	m_type_parts := strings.Split(m_type, "/")
	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to generate random filename", err)
		return
	}
	name := base64.RawURLEncoding.EncodeToString(key)
	file_name := fmt.Sprintf("%s.%s", name, m_type_parts[1])
	file_path := filepath.Join(cfg.assetsRoot, file_name)
	file_ptr, err := os.Create(file_path)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to store thumbnail", err)
		return
	}
	defer file_ptr.Close()
	reader := io.Reader(file)
	writer := io.Writer(file_ptr)
	_, err = io.Copy(writer, reader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to store thumbnail", err)
		return
	}
	tn_url := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, file_name)
	vide_metadata.ThumbnailURL = &tn_url

	err = cfg.db.UpdateVideo(vide_metadata)
	if err != nil {
		delete(videoThumbnails, videoID)
		respondWithError(w, http.StatusBadRequest, "Unable to update metadata in database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, vide_metadata)
}
