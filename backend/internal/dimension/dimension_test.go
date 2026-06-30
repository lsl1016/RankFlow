package dimension

import (
	"testing"
	"time"

	"rankflow/internal/model"
)

func TestComputeNoTimeNoDims(t *testing.T) {
	tc := &model.RankTimeConfig{TimeType: model.TimeNone}
	got, err := Compute(tc, nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "global" {
		t.Fatalf("want global, got %q", got)
	}
}

func TestComputeDimsConcatInOrder(t *testing.T) {
	tc := &model.RankTimeConfig{TimeType: model.TimeNone}
	dims := []model.RankDimensionConfig{
		{DimensionField: "business_id", DimensionOrder: 0, Required: 1},
		{DimensionField: "category_id", DimensionOrder: 1, Required: 1},
	}
	got, err := Compute(tc, dims, map[string]string{"business_id": "community", "category_id": "tech"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "community_tech" {
		t.Fatalf("want community_tech, got %q", got)
	}
}

func TestComputeMissingRequiredDimErrors(t *testing.T) {
	tc := &model.RankTimeConfig{TimeType: model.TimeNone}
	dims := []model.RankDimensionConfig{{DimensionField: "business_id", Required: 1}}
	if _, err := Compute(tc, dims, map[string]string{}, 0); err == nil {
		t.Fatal("expected error for missing required dimension")
	}
}

func TestComputeDayBucketStable(t *testing.T) {
	tc := &model.RankTimeConfig{TimeType: model.TimeDay, Timezone: "Asia/Shanghai"}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	morning := time.Date(2026, 6, 30, 8, 0, 0, 0, loc).Unix()
	evening := time.Date(2026, 6, 30, 22, 0, 0, 0, loc).Unix()
	a, _ := Compute(tc, nil, nil, morning)
	b, _ := Compute(tc, nil, nil, evening)
	if a != b {
		t.Fatalf("same-day timestamps must share a bucket: %q vs %q", a, b)
	}
	nextDay := time.Date(2026, 7, 1, 1, 0, 0, 0, loc).Unix()
	c, _ := Compute(tc, nil, nil, nextDay)
	if c == a {
		t.Fatalf("different day must produce different bucket")
	}
}
