package score

import (
	"testing"

	"rankflow/internal/model"
)

func TestSubDecimalEarlyFirstDesc(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreEarlyFirst}
	early := SubDecimal(cfg, 1000, 0)
	late := SubDecimal(cfg, 2000, 0)
	if !(early > late) { t.Fatalf("early event should outrank later: early=%v late=%v", early, late) }
}

func TestSubScoreNeverBreaksPrimaryOrdering(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreSubScore}
	high := Final(cfg, 100, 0, -200_000_000_000)
	low := Final(cfg, 99, 0, 0)
	if !(high > low) { t.Fatalf("higher primary score must win: high=%v low=%v", high, low) }
}

func TestEncodedMemberPreservesOneSecondTieAtLargePrimaryScore(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreEarlyFirst}
	early := EncodedMember(cfg, 1_700_000_000, 0, "early")
	late := EncodedMember(cfg, 1_700_000_001, 0, "late")
	if !(early > late) {
		t.Fatalf("descending equal-score lexicographic order must prefer earlier second: early=%q late=%q", early, late)
	}
}

func TestEncodedMemberSubScoreSignedOrder(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreDesc, SameScorePolicy: model.SameScoreSubScore}
	neg := EncodedMember(cfg, 0, -10, "neg")
	zero := EncodedMember(cfg, 0, 0, "zero")
	pos := EncodedMember(cfg, 0, 10, "pos")
	if !(pos > zero && zero > neg) { t.Fatalf("signed sub score ordering broken: %q %q %q", neg, zero, pos) }
}

func TestAscendingTieKeyIsInverted(t *testing.T) {
	cfg := &model.RankConfig{SortType: model.SortScoreAsc, SameScorePolicy: model.SameScoreEarlyFirst}
	early := EncodedMember(cfg, 1000, 0, "early")
	late := EncodedMember(cfg, 2000, 0, "late")
	if !(early < late) { t.Fatalf("ascending ZRANGE must still prefer earlier event: %q %q", early, late) }
}
