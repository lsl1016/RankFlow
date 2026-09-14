package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"rankflow/internal/dimension"
	"rankflow/internal/model"
	"rankflow/internal/observability"
	"rankflow/internal/score"
	redisstore "rankflow/internal/store/redis"
)

type PersistJob struct {
	RankID    int64   `json:"rankId"`
	TypeID    string  `json:"typeId"`
	ItemID    string  `json:"itemId"`
	TraceID   string  `json:"traceId,omitempty"`
	Score     int64   `json:"score"`
	SubScore  int64   `json:"subScore"`
	Final     float64 `json:"final"`
	Revision  int64   `json:"revision"`
	EventTime int64   `json:"eventTime"`
}

type AddScoreInput struct {
	RequestID  string            `json:"requestId"`
	ItemID     string            `json:"itemId"`
	Score      int64             `json:"score"`
	SubScore   int64             `json:"subScore"`
	EventTime  int64             `json:"eventTime"`
	Dimensions map[string]string `json:"dimensions"`
}

type ScoreResult struct {
	RankID int64   `json:"rankId"`
	TypeID string  `json:"typeId"`
	ItemID string  `json:"itemId"`
	Score  int64   `json:"score"`
	Rank   int     `json:"rank"`
	Final  float64 `json:"final"`
}

type writeFingerprintPayload struct {
	Operation  string            `json:"operation"`
	ItemID     string            `json:"itemId"`
	Score      int64             `json:"score"`
	SubScore   int64             `json:"subScore"`
	EventTime  int64             `json:"eventTime"`
	Dimensions map[string]string `json:"dimensions"`
}

func (s *Service) anchorTS(rc *ResolvedConfig, eventTS int64) int64 {
	if rc.Time.AnchorType == model.AnchorRequestTime || eventTS <= 0 {
		return time.Now().Unix()
	}
	return eventTS
}

// prepareBoard first migrates the old Redis index. If Redis was flushed or the
// v2 index is otherwise missing, it rebuilds the ranking from persisted MySQL
// member state. RestoreMemberIfNewer makes the rebuild safe against a concurrent
// newer Redis write.
func (s *Service) prepareBoard(ctx context.Context, rc *ResolvedConfig, rankID int64, typeID string) error {
	if err := s.rd.MigrateLegacyBoard(ctx, rankID, typeID, &rc.Config); err != nil {
		return err
	}
	hasBoard, err := s.rd.HasBoard(ctx, rankID, typeID)
	if err != nil || hasBoard {
		return err
	}
	rows, err := s.my.ListMemberScores(ctx, rankID, typeID)
	if err != nil {
		return err
	}
	for i := range rows {
		row := &rows[i]
		anchor := int64(0)
		if row.LastEventTime != nil {
			anchor = row.LastEventTime.Unix()
		}
		encodedMember := score.EncodedMember(&rc.Config, anchor, row.SubScore, row.ItemID)
		if err := s.rd.RestoreMemberIfNewer(ctx, rankID, typeID, row.ItemID, encodedMember, row.Score, row.Revision, row.FinalScore); err != nil {
			return err
		}
	}
	return s.rd.TrimBoard(ctx, rankID, typeID, rc.Config.MaxRankSize, score.IsDesc(&rc.Config))
}

func writeFingerprint(operation string, in *AddScoreInput) (string, error) {
	payload, err := json.Marshal(writeFingerprintPayload{
		Operation:  operation,
		ItemID:     in.ItemID,
		Score:      in.Score,
		SubScore:   in.SubScore,
		EventTime:  in.EventTime,
		Dimensions: in.Dimensions,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) existingWriteResult(ctx context.Context, rankID int64, requestID, fingerprint string) (*ScoreResult, bool, error) {
	if requestID == "" {
		return nil, false, nil
	}
	record, found, err := s.rd.GetIdempotencyRecord(ctx, rankID, requestID)
	if errors.Is(err, redisstore.ErrLegacyIdempotencyRecord) {
		return nil, true, fmt.Errorf("%w: requestId %q was claimed by a previous RankFlow version", ErrIdempotencyConflict, requestID)
	}
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}
	if record.Fingerprint != fingerprint {
		return nil, true, fmt.Errorf("%w: requestId %q was already used for a different write", ErrIdempotencyConflict, requestID)
	}
	rc, err := s.resolve(ctx, rankID)
	if err != nil {
		return nil, true, err
	}
	if err := s.prepareBoard(ctx, rc, rankID, record.TypeID); err != nil {
		return nil, true, err
	}
	rank, _, err := s.rd.MemberRank(ctx, rankID, record.TypeID, record.ItemID, score.IsDesc(&rc.Config))
	if err != nil {
		return nil, true, err
	}
	return &ScoreResult{
		RankID: rankID,
		TypeID: record.TypeID,
		ItemID: record.ItemID,
		Score:  record.Score,
		Rank:   rank,
		Final:  record.Final,
	}, true, nil
}

func (s *Service) writeScore(ctx context.Context, rankID int64, mode string, in *AddScoreInput) (*ScoreResult, error) {
	if in.ItemID == "" {
		return nil, fmt.Errorf("%w: itemId is required", ErrValidation)
	}
	if mode == redisstore.WriteModeAdd && strings.TrimSpace(in.RequestID) == "" {
		return nil, fmt.Errorf("%w: requestId is required for additive score writes", ErrValidation)
	}
	if len(in.ItemID) > 128 {
		return nil, fmt.Errorf("%w: itemId exceeds 128 characters", ErrValidation)
	}
	if len(in.RequestID) > 256 {
		return nil, fmt.Errorf("%w: requestId exceeds 256 characters", ErrValidation)
	}
	fingerprint, err := writeFingerprint(mode, in)
	if err != nil {
		return nil, err
	}
	if result, found, err := s.existingWriteResult(ctx, rankID, in.RequestID, fingerprint); found || err != nil {
		return result, err
	}

	rc, err := s.resolve(ctx, rankID)
	if err != nil {
		return nil, err
	}
	if rc.Config.Status != model.StatusOnline {
		return nil, ErrNotOnline
	}
	if mode == redisstore.WriteModeSet && (in.Score > score.MaxExactBusinessScore || in.Score < -score.MaxExactBusinessScore) {
		return nil, fmt.Errorf("%w: score exceeds exact Redis range", ErrValidation)
	}

	anchor := s.anchorTS(rc, in.EventTime)
	typeID, err := dimension.Compute(&rc.Time, rc.Dimensions, in.Dimensions, anchor)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	if err := s.requireSubBoardOnline(ctx, rankID, typeID, in.Dimensions); err != nil {
		return nil, err
	}
	if err := s.prepareBoard(ctx, rc, rankID, typeID); err != nil {
		return nil, err
	}

	atomicResult, err := s.rd.WriteScoreAtomic(ctx, redisstore.AtomicWriteRequest{
		Mode:           mode,
		RankID:         rankID,
		TypeID:         typeID,
		ItemID:         in.ItemID,
		EncodedMember:  score.EncodedMember(&rc.Config, anchor, in.SubScore, in.ItemID),
		Value:          in.Score,
		SubScore:       in.SubScore,
		EventTime:      anchor,
		SubDecimal:     score.SubDecimal(&rc.Config, anchor, in.SubScore),
		MaxSize:        rc.Config.MaxRankSize,
		SortDesc:       score.IsDesc(&rc.Config),
		RequestID:      in.RequestID,
		Fingerprint:    fingerprint,
		IdempotencyTTL: 24 * time.Hour,
		TraceID:        observability.TraceID(ctx),
	})
	if errors.Is(err, redisstore.ErrLegacyIdempotencyRecord) {
		return nil, fmt.Errorf("%w: requestId %q was claimed by a previous RankFlow version", ErrIdempotencyConflict, in.RequestID)
	}
	if err != nil {
		if strings.Contains(err.Error(), "score exceeds exact Redis range") {
			return nil, fmt.Errorf("%w: score exceeds exact Redis range", ErrValidation)
		}
		return nil, err
	}
	if atomicResult.Conflict {
		return nil, fmt.Errorf("%w: requestId %q was already used for a different write", ErrIdempotencyConflict, in.RequestID)
	}

	resultTypeID := atomicResult.TypeID
	resultItemID := atomicResult.ItemID
	if resultTypeID == "" {
		resultTypeID = typeID
	}
	if resultItemID == "" {
		resultItemID = in.ItemID
	}
	// A concurrent duplicate can cross a request-time bucket boundary. In that
	// case the Lua script returns the first write's stored typeId, so query the
	// original board rather than the retry's newly computed one.
	if resultTypeID != typeID {
		if err := s.prepareBoard(ctx, rc, rankID, resultTypeID); err != nil {
			return nil, err
		}
	}
	rank, _, err := s.rd.MemberRank(ctx, rankID, resultTypeID, resultItemID, score.IsDesc(&rc.Config))
	if err != nil {
		return nil, err
	}
	s.logger(ctx).Debug("score write succeeded",
		zap.Int64("rankId", rankID),
		zap.String("typeId", resultTypeID),
		zap.String("itemId", resultItemID),
		zap.String("mode", mode),
		zap.Bool("duplicate", atomicResult.Duplicate),
		zap.Int64("score", atomicResult.Score),
		zap.Int64("revision", atomicResult.Revision),
		zap.Int("rank", rank),
	)
	return &ScoreResult{
		RankID: rankID,
		TypeID: resultTypeID,
		ItemID: resultItemID,
		Score:  atomicResult.Score,
		Rank:   rank,
		Final:  atomicResult.Final,
	}, nil
}

func (s *Service) AddScore(ctx context.Context, rankID int64, in *AddScoreInput) (*ScoreResult, error) {
	return s.writeScore(ctx, rankID, redisstore.WriteModeAdd, in)
}

func (s *Service) SetScore(ctx context.Context, rankID int64, in *AddScoreInput) (*ScoreResult, error) {
	return s.writeScore(ctx, rankID, redisstore.WriteModeSet, in)
}

func (s *Service) BatchAddScore(ctx context.Context, rankID int64, items []AddScoreInput) ([]ScoreResult, error) {
	results := make([]ScoreResult, 0, len(items))
	for i := range items {
		r, err := s.AddScore(ctx, rankID, &items[i])
		if err != nil {
			return results, err
		}
		results = append(results, *r)
	}
	return results, nil
}
