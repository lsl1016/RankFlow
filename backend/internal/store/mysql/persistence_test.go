package mysql

import (
	"context"
	"testing"
	"time"

	"rankflow/internal/model"
)

func TestUpsertMemberScoreIfNewerRejectsStaleRevision(t *testing.T) {
	store := testMySQLStore(t)
	ctx := context.Background()
	rankID := time.Now().UnixNano()
	typeID := "global"
	itemID := "user_revision"
	now := time.Now()

	t.Cleanup(func() {
		store.DB().Where("rank_id = ?", rankID).Delete(&model.RankMemberScore{})
	})

	newer := &model.RankMemberScore{
		RankID: rankID, TypeID: typeID, ItemID: itemID,
		Score: 20, SubScore: 2, FinalScore: 20.2, Revision: 2,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := store.UpsertMemberScoreIfNewer(ctx, newer); err != nil {
		t.Fatalf("persist newer revision: %v", err)
	}

	stale := &model.RankMemberScore{
		RankID: rankID, TypeID: typeID, ItemID: itemID,
		Score: 10, SubScore: 1, FinalScore: 10.1, Revision: 1,
		CreatedAt: now, UpdatedAt: now.Add(time.Second),
	}
	if err := store.UpsertMemberScoreIfNewer(ctx, stale); err != nil {
		t.Fatalf("persist stale revision: %v", err)
	}

	var got model.RankMemberScore
	if err := store.DB().Where("rank_id = ? AND type_id = ? AND item_id = ?", rankID, typeID, itemID).First(&got).Error; err != nil {
		t.Fatalf("load member score: %v", err)
	}
	if got.Score != 20 || got.Revision != 2 || got.SubScore != 2 {
		t.Fatalf("stale revision overwrote newer state: %+v", got)
	}
}
