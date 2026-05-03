package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"soundcloud/internal/domain"
)

const soundCloudProvider = "soundcloud"

var errSoundCloudAlbumURL = errors.New("soundcloud album or playlist link is required")

type soundCloudInfo struct {
	ID         string           `json:"id"`
	Title      string           `json:"title"`
	Uploader   string           `json:"uploader"`
	Artist     string           `json:"artist"`
	Creator    string           `json:"creator"`
	Thumbnail  string           `json:"thumbnail"`
	WebpageURL string           `json:"webpage_url"`
	URL        string           `json:"url"`
	Entries    []soundCloudInfo `json:"entries"`
}

type downloadedSoundCloudTrack struct {
	info soundCloudInfo
	path string
	size int64
}

func validateSoundCloudURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", errors.New("soundcloud url is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid soundcloud url")
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", errors.New("invalid soundcloud url")
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "soundcloud.com" && host != "www.soundcloud.com" && host != "on.soundcloud.com" {
		return "", errors.New("only soundcloud links are supported")
	}

	parsed.Fragment = ""
	return parsed.String(), nil
}

func isSoundCloudAlbumURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	for _, part := range strings.Split(strings.Trim(path.Clean(parsed.Path), "/"), "/") {
		if part == "sets" {
			return true
		}
	}
	return false
}

func soundCloudBinary(binary string) string {
	binary = strings.TrimSpace(binary)
	if binary == "" {
		return "yt-dlp"
	}
	return binary
}

func soundCloudInfoURL(info soundCloudInfo, fallback string) string {
	if strings.TrimSpace(info.WebpageURL) != "" {
		return strings.TrimSpace(info.WebpageURL)
	}
	if strings.TrimSpace(info.URL) != "" && strings.HasPrefix(info.URL, "http") {
		return strings.TrimSpace(info.URL)
	}
	return fallback
}

func soundCloudArtist(info soundCloudInfo) string {
	for _, value := range []string{info.Artist, info.Uploader, info.Creator} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "SoundCloud"
}

func soundCloudTitle(info soundCloudInfo) string {
	if title := strings.TrimSpace(info.Title); title != "" {
		return title
	}
	return "SoundCloud track"
}

func soundCloudAlbumDescription(info soundCloudInfo) string {
	artist := soundCloudArtist(info)
	if artist == "SoundCloud" {
		return "Импортировано из SoundCloud"
	}
	return "SoundCloud: " + artist
}

func fetchSoundCloudInfo(ctx context.Context, binary, rawURL string, noPlaylist bool) (soundCloudInfo, string, error) {
	canonicalURL, err := validateSoundCloudURL(rawURL)
	if err != nil {
		return soundCloudInfo{}, "", err
	}

	args := []string{"--dump-single-json", "--no-warnings"}
	if noPlaylist {
		args = append(args, "--no-playlist")
	}
	args = append(args, canonicalURL)

	cmdCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	output, err := exec.CommandContext(cmdCtx, soundCloudBinary(binary), args...).Output()
	if err != nil {
		return soundCloudInfo{}, "", fmt.Errorf("failed to fetch soundcloud metadata with yt-dlp: %w", err)
	}

	var info soundCloudInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return soundCloudInfo{}, "", errors.New("failed to parse soundcloud metadata")
	}

	return info, soundCloudInfoURL(info, canonicalURL), nil
}

func downloadSoundCloudTrack(ctx context.Context, binary, sourceURL, trackID string) (downloadedSoundCloudTrack, error) {
	info, canonicalURL, err := fetchSoundCloudInfo(ctx, binary, sourceURL, true)
	if err != nil {
		return downloadedSoundCloudTrack{}, err
	}
	if len(info.Entries) > 0 || isSoundCloudAlbumURL(canonicalURL) {
		return downloadedSoundCloudTrack{}, errors.New("use album import for soundcloud album or playlist links")
	}

	tmpDir, err := os.MkdirTemp("", "soundcloud-import-*")
	if err != nil {
		return downloadedSoundCloudTrack{}, err
	}

	outputTemplate := filepath.Join(tmpDir, trackID+".%(ext)s")
	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", outputTemplate,
		canonicalURL,
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	output, err := exec.CommandContext(cmdCtx, soundCloudBinary(binary), args...).CombinedOutput()
	if err != nil {
		os.RemoveAll(tmpDir)
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return downloadedSoundCloudTrack{}, fmt.Errorf("failed to download soundcloud track: %s", message)
	}

	matches, err := filepath.Glob(filepath.Join(tmpDir, trackID+".mp3"))
	if err != nil || len(matches) == 0 {
		os.RemoveAll(tmpDir)
		return downloadedSoundCloudTrack{}, errors.New("downloaded soundcloud audio file was not found")
	}

	stat, err := os.Stat(matches[0])
	if err != nil {
		os.RemoveAll(tmpDir)
		return downloadedSoundCloudTrack{}, err
	}
	if stat.Size() == 0 {
		os.RemoveAll(tmpDir)
		return downloadedSoundCloudTrack{}, errors.New("downloaded soundcloud audio file is empty")
	}
	if stat.Size() > 100<<20 {
		os.RemoveAll(tmpDir)
		return downloadedSoundCloudTrack{}, errors.New("downloaded file is too large, max is 100MB")
	}

	info.WebpageURL = canonicalURL
	return downloadedSoundCloudTrack{info: info, path: matches[0], size: stat.Size()}, nil
}

func saveDownloadedSoundCloudTrack(ctx context.Context, s *TrackService, ownerID, albumID, trackID string, downloaded downloadedSoundCloudTrack) (domain.Track, error) {
	file, err := os.Open(downloaded.path)
	if err != nil {
		return domain.Track{}, err
	}
	defer file.Close()

	filename := trackID + ".mp3"
	storageKey, err := s.storage.Save(ctx, trackID, filename, file)
	if err != nil {
		return domain.Track{}, err
	}

	track := domain.Track{
		ID:             trackID,
		OwnerID:        ownerID,
		AlbumID:        albumID,
		Title:          soundCloudTitle(downloaded.info),
		Artist:         soundCloudArtist(downloaded.info),
		Filename:       filename,
		ContentType:    "audio/mpeg",
		Size:           downloaded.size,
		StorageKey:     storageKey,
		ArtworkURL:     strings.TrimSpace(downloaded.info.Thumbnail),
		SourceProvider: soundCloudProvider,
		SourceURL:      soundCloudInfoURL(downloaded.info, ""),
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.tracks.Create(ctx, track); err != nil {
		return domain.Track{}, err
	}

	return track, nil
}

func (s *TrackService) ImportSoundCloud(ctx context.Context, ownerID, sourceURL, albumID string) (domain.Track, error) {
	albumID = strings.TrimSpace(albumID)

	if _, err := s.users.FindByID(ctx, ownerID); err != nil {
		return domain.Track{}, err
	}

	if albumID != "" {
		album, err := s.albums.FindByID(ctx, albumID)
		if err != nil {
			return domain.Track{}, err
		}
		if album.OwnerID != ownerID {
			return domain.Track{}, errors.New("album does not belong to user")
		}
	}

	trackID := newID()
	downloaded, err := downloadSoundCloudTrack(ctx, s.ytdlp, sourceURL, trackID)
	if err != nil {
		return domain.Track{}, err
	}
	defer os.RemoveAll(filepath.Dir(downloaded.path))

	return saveDownloadedSoundCloudTrack(ctx, s, ownerID, albumID, trackID, downloaded)
}

func (s *AlbumService) ImportSoundCloud(ctx context.Context, ownerID, sourceURL string) (domain.Album, error) {
	if _, err := s.users.FindByID(ctx, ownerID); err != nil {
		return domain.Album{}, err
	}

	info, canonicalURL, err := fetchSoundCloudInfo(ctx, s.ytdlp, sourceURL, false)
	if err != nil {
		return domain.Album{}, err
	}
	if len(info.Entries) == 0 && !isSoundCloudAlbumURL(canonicalURL) {
		return domain.Album{}, errSoundCloudAlbumURL
	}
	if len(info.Entries) == 0 {
		return domain.Album{}, errors.New("soundcloud album has no tracks")
	}

	album := domain.Album{
		ID:             newID(),
		OwnerID:        ownerID,
		Title:          soundCloudTitle(info),
		Description:    soundCloudAlbumDescription(info),
		ArtworkURL:     strings.TrimSpace(info.Thumbnail),
		SourceProvider: soundCloudProvider,
		SourceURL:      canonicalURL,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.albums.Create(ctx, album); err != nil {
		return domain.Album{}, err
	}

	trackService := &TrackService{tracks: s.tracks, users: s.users, albums: s.albums, storage: s.store, ytdlp: s.ytdlp}
	for index, entry := range info.Entries {
		entryURL := soundCloudInfoURL(entry, "")
		if entryURL == "" {
			continue
		}

		trackID := newID()
		downloaded, err := downloadSoundCloudTrack(ctx, s.ytdlp, entryURL, trackID)
		if err != nil {
			return domain.Album{}, fmt.Errorf("failed to import album track %d: %w", index+1, err)
		}
		_, saveErr := saveDownloadedSoundCloudTrack(ctx, trackService, ownerID, album.ID, trackID, downloaded)
		os.RemoveAll(filepath.Dir(downloaded.path))
		if saveErr != nil {
			return domain.Album{}, fmt.Errorf("failed to save album track %d: %w", index+1, saveErr)
		}
	}

	return album, nil
}
