package queue

import (
	"encoding/json"
	"testing"

	"rankflow/internal/service"
	redisstore "rankflow/internal/store/redis"
)

func TestDecodePersistJobV2(t *testing.T) {
	msg := redisstore.PersistStreamMessage{
		ID: "1-0",
		Values: map[string]string{
			"format":    "v2",
			"rankId":    "10001",
			"typeId":    "global",
			"itemId":    "user_1",
			"traceId":   "trace-1",
			"score":     "42",
			"subScore":  "7",
			"final":     "42.5",
			"revision":  "3",
			"eventTime": "1700000000",
		},
	}
	job, err := decodePersistJob(msg)
	if err != nil {
		t.Fatalf("decode v2 message: %v", err)
	}
	if job.RankID != 10001 || job.TypeID != "global" || job.ItemID != "user_1" || job.Score != 42 || job.SubScore != 7 || job.Revision != 3 || job.EventTime != 1700000000 || job.Final != 42.5 {
		t.Fatalf("unexpected v2 job: %#v", job)
	}
}

func TestDecodePersistJobLegacyPayload(t *testing.T) {
	want := service.PersistJob{RankID: 10002, TypeID: "day_20260915", ItemID: "user_2", Score: 9, Revision: 2}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	job, err := decodePersistJob(redisstore.PersistStreamMessage{ID: "2-0", Payload: string(payload)})
	if err != nil {
		t.Fatalf("decode legacy payload: %v", err)
	}
	if job.RankID != want.RankID || job.TypeID != want.TypeID || job.ItemID != want.ItemID || job.Score != want.Score || job.Revision != want.Revision {
		t.Fatalf("unexpected legacy job: %#v", job)
	}
}
