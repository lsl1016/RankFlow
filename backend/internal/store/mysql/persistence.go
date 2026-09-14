package mysql

import (
	"context"

	"rankflow/internal/model"
)

// UpsertMemberScoreIfNewer applies a persisted member state only when its
// revision is newer than the row already stored. This prevents concurrent
// persist workers from letting an older Redis event overwrite a newer score.
func (s *Store) UpsertMemberScoreIfNewer(ctx context.Context, m *model.RankMemberScore) error {
	return s.db.WithContext(ctx).Exec(`
INSERT INTO rank_member_score
(rank_id, type_id, item_id, score, sub_score, final_score, revision, rank_no, last_event_time, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  score = IF(VALUES(revision) > revision, VALUES(score), score),
  sub_score = IF(VALUES(revision) > revision, VALUES(sub_score), sub_score),
  final_score = IF(VALUES(revision) > revision, VALUES(final_score), final_score),
  last_event_time = IF(VALUES(revision) > revision, VALUES(last_event_time), last_event_time),
  updated_at = IF(VALUES(revision) > revision, VALUES(updated_at), updated_at),
  revision = GREATEST(revision, VALUES(revision))`,
		m.RankID, m.TypeID, m.ItemID, m.Score, m.SubScore, m.FinalScore, m.Revision,
		m.RankNo, m.LastEventTime, m.CreatedAt, m.UpdatedAt,
	).Error
}

func (s *Store) ListMemberScores(ctx context.Context, rankID int64, typeID string) ([]model.RankMemberScore, error) {
	var rows []model.RankMemberScore
	err := s.db.WithContext(ctx).
		Where("rank_id = ? AND type_id = ?", rankID, typeID).
		Order("id asc").Find(&rows).Error
	return rows, err
}
