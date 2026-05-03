package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

type downloadedSoundCloudAlbumResult struct {
	tracks   []downloadedSoundCloudTrack
	failures []error
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

type soundCloudFetchOptions struct {
	noPlaylist   bool
	flatPlaylist bool
	timeout      time.Duration
}

func fetchSoundCloudInfo(ctx context.Context, binary, rawURL string, opts soundCloudFetchOptions) (soundCloudInfo, string, error) {
	canonicalURL, err := validateSoundCloudURL(rawURL)
	if err != nil {
		return soundCloudInfo{}, "", err
	}

	args := []string{"--dump-single-json", "--no-warnings"}
	if opts.noPlaylist {
		args = append(args, "--no-playlist")
	}
	if opts.flatPlaylist {
		args = append(args, "--flat-playlist")
	}
	args = append(args, canonicalURL)

	timeout := opts.timeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, soundCloudBinary(binary), args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return soundCloudInfo{}, "", err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return soundCloudInfo{}, "", fmt.Errorf("failed to start yt-dlp: %w", err)
	}

	output, readErr := io.ReadAll(stdout)
	waitErr := cmd.Wait()

	if readErr != nil {
		return soundCloudInfo{}, "", fmt.Errorf("failed to read soundcloud metadata output: %w", readErr)
	}
	if waitErr != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return soundCloudInfo{}, "", fmt.Errorf("failed to fetch soundcloud metadata with yt-dlp: timed out after %s", timeout)
		}

		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(string(output))
		}
		if message == "" {
			message = waitErr.Error()
		}
		return soundCloudInfo{}, "", fmt.Errorf("failed to fetch soundcloud metadata with yt-dlp: %s", message)
	}

	var info soundCloudInfo
	if err := json.Unmarshal(bytes.TrimSpace(output), &info); err != nil {
		snippet := strings.TrimSpace(string(output))
		if len(snippet) > 240 {
			snippet = snippet[:240]
		}
		if snippet == "" {
			snippet = strings.TrimSpace(stderr.String())
		}
		if snippet == "" {
			snippet = err.Error()
		}
		return soundCloudInfo{}, "", fmt.Errorf("failed to parse soundcloud metadata: %s", snippet)
	}

	return info, soundCloudInfoURL(info, canonicalURL), nil
}

func downloadSoundCloudTrack(ctx context.Context, binary, sourceURL, trackID string) (downloadedSoundCloudTrack, error) {
	info, canonicalURL, err := fetchSoundCloudInfo(ctx, binary, sourceURL, soundCloudFetchOptions{
		noPlaylist: true,
		timeout:    45 * time.Second,
	})
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

func downloadSoundCloudAlbumTracks(ctx context.Context, binary, sourceURL string, entries []soundCloudInfo) (downloadedSoundCloudAlbumResult, func(), error) {
	tmpDir, err := os.MkdirTemp("", "soundcloud-album-import-*")
	if err != nil {
		return downloadedSoundCloudAlbumResult{}, nil, err
	}

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	outputTemplate := filepath.Join(tmpDir, "%(playlist_index)s-%(id)s.%(ext)s")
	args := []string{
		"--yes-playlist",
		"--no-warnings",
		"--ignore-errors",
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"-o", outputTemplate,
		sourceURL,
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	output, cmdErr := exec.CommandContext(cmdCtx, soundCloudBinary(binary), args...).CombinedOutput()
	filesByTrackID := make(map[string]string, len(entries))

	walkErr := filepath.WalkDir(tmpDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.ToLower(filepath.Ext(filePath)) != ".mp3" {
			return err
		}

		base := strings.TrimSuffix(filepath.Base(filePath), ".mp3")
		parts := strings.SplitN(base, "-", 2)
		if len(parts) != 2 {
			return nil
		}

		filesByTrackID[parts[1]] = filePath
		return nil
	})
	if walkErr != nil {
		cleanup()
		return downloadedSoundCloudAlbumResult{}, nil, walkErr
	}

	result := downloadedSoundCloudAlbumResult{
		tracks:   make([]downloadedSoundCloudTrack, 0, len(entries)),
		failures: make([]error, 0),
	}

	for index, entry := range entries {
		trackPath, ok := filesByTrackID[strings.TrimSpace(entry.ID)]
		if !ok {
			result.failures = append(result.failures, fmt.Errorf("track %d was not downloaded", index+1))
			continue
		}

		stat, err := os.Stat(trackPath)
		if err != nil {
			result.failures = append(result.failures, fmt.Errorf("track %d stat failed: %w", index+1, err))
			continue
		}
		if stat.Size() == 0 {
			result.failures = append(result.failures, fmt.Errorf("track %d file is empty", index+1))
			continue
		}
		if stat.Size() > 100<<20 {
			result.failures = append(result.failures, fmt.Errorf("track %d file is too large", index+1))
			continue
		}

		result.tracks = append(result.tracks, downloadedSoundCloudTrack{
			info: entry,
			path: trackPath,
			size: stat.Size(),
		})
	}

	if len(result.tracks) == 0 {
		cleanup()
		message := strings.TrimSpace(string(output))
		if message == "" && cmdErr != nil {
			message = cmdErr.Error()
		}
		if message == "" {
			message = "no album tracks were downloaded"
		}
		return downloadedSoundCloudAlbumResult{}, nil, fmt.Errorf("failed to download soundcloud album: %s", message)
	}

	return result, cleanup, nil
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

	info, canonicalURL, err := fetchSoundCloudInfo(ctx, s.ytdlp, sourceURL, soundCloudFetchOptions{
		flatPlaylist: true,
		timeout:      3 * time.Minute,
	})
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
	downloadedAlbum, cleanup, err := downloadSoundCloudAlbumTracks(ctx, s.ytdlp, canonicalURL, info.Entries)
	if err != nil {
		return domain.Album{}, err
	}
	defer cleanup()

	importedCount := 0
	for _, downloaded := range downloadedAlbum.tracks {
		trackID := newID()
		if strings.TrimSpace(downloaded.info.Thumbnail) == "" {
			downloaded.info.Thumbnail = info.Thumbnail
		}
		if strings.TrimSpace(downloaded.info.Artist) == "" && strings.TrimSpace(downloaded.info.Uploader) == "" && strings.TrimSpace(downloaded.info.Creator) == "" {
			downloaded.info.Artist = soundCloudArtist(info)
		}
		if strings.TrimSpace(downloaded.info.WebpageURL) == "" {
			downloaded.info.WebpageURL = soundCloudInfoURL(downloaded.info, canonicalURL)
		}
		if _, err := saveDownloadedSoundCloudTrack(ctx, trackService, ownerID, album.ID, trackID, downloaded); err != nil {
			downloadedAlbum.failures = append(downloadedAlbum.failures, fmt.Errorf("failed to save %q: %w", soundCloudTitle(downloaded.info), err))
			continue
		}
		importedCount++
	}

	if importedCount == 0 {
		return domain.Album{}, errors.New("soundcloud album import failed: no tracks were saved")
	}

	return album, nil
}
