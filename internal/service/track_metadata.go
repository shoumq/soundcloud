package service

import (
	"context"

	"soundcloud/internal/domain"
	"soundcloud/internal/repository"
)

func enrichTracksWithLikes(ctx context.Context, repo repository.TrackRepository, viewerID string, tracks []domain.Track) ([]domain.Track, error) {
	if len(tracks) == 0 {
		return tracks, nil
	}

	trackIDs := make([]string, 0, len(tracks))
	for _, track := range tracks {
		if track.ID != "" {
			trackIDs = append(trackIDs, track.ID)
		}
	}

	likeCounts, err := repo.CountLikesByTrackIDs(ctx, trackIDs)
	if err != nil {
		return nil, err
	}

	likedTrackIDs, err := repo.ListLikedTrackIDs(ctx, viewerID, trackIDs)
	if err != nil {
		return nil, err
	}

	enriched := make([]domain.Track, 0, len(tracks))
	for _, track := range tracks {
		track.LikesCount = likeCounts[track.ID]
		_, track.LikedByMe = likedTrackIDs[track.ID]
		enriched = append(enriched, track)
	}

	return enriched, nil
}
