package dimension

import (
	"fmt"
	"strings"
	"time"

	"rankflow/internal/model"
)

const missingDimensionToken = "~"

// Compute builds the type_id for a sub-leaderboard from the time bucket and the
// configured business dimensions. Layout: {time_bucket}_{dim1}_{dim2}_...
//
// When time granularity is "none" the time bucket is omitted. Dimensions are
// concatenated in DimensionOrder. Missing required dimensions cause an error.
func Compute(tc *model.RankTimeConfig, dims []model.RankDimensionConfig, dimValues map[string]string, anchorTS int64) (string, error) {
	var parts []string

	if tc != nil && tc.TimeType != model.TimeNone && tc.TimeType != "" {
		bucket, err := timeBucket(tc, anchorTS)
		if err != nil {
			return "", err
		}
		parts = append(parts, bucket)
	}

	for _, d := range dims {
		v := dimValues[d.DimensionField]
		if v == "" {
			if d.Required == 1 {
				return "", fmt.Errorf("missing required dimension %q", d.DimensionField)
			}
			parts = append(parts, missingDimensionToken)
			continue
		}
		parts = append(parts, encodeDimensionValue(v))
	}

	if len(parts) == 0 {
		return "global", nil
	}
	return strings.Join(parts, "_"), nil
}

// timeBucket returns the bucket label for the anchor timestamp in the rank's
// timezone. Uses the start-of-period epoch second so buckets are stable and
// machine-friendly.
func timeBucket(tc *model.RankTimeConfig, anchorTS int64) (string, error) {
	loc, err := time.LoadLocation(tc.Timezone)
	if err != nil {
		return "", fmt.Errorf("invalid timezone %q: %w", tc.Timezone, err)
	}
	if anchorTS <= 0 {
		anchorTS = time.Now().Unix()
	}
	t := time.Unix(anchorTS, 0).In(loc)

	switch tc.TimeType {
	case model.TimeHour:
		start := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, loc)
		return fmt.Sprint(start.Unix()), nil
	case model.TimeDay:
		start := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		return fmt.Sprint(start.Unix()), nil
	case model.TimeWeek:
		weekday := int(t.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(weekday - 1))
		return fmt.Sprint(monday.Unix()), nil
	case model.TimeMonth:
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
		return fmt.Sprint(start.Unix()), nil
	case model.TimeSeason:
		q := (int(t.Month()) - 1) / 3
		startMonth := time.Month(q*3 + 1)
		start := time.Date(t.Year(), startMonth, 1, 0, 0, 0, 0, loc)
		return fmt.Sprint(start.Unix()), nil
	case model.TimeCustom:
		return "custom", nil
	default:
		return "", fmt.Errorf("unsupported time type %q", tc.TimeType)
	}
}

// encodeDimensionValue escapes characters that have structural meaning in a
// type_id. It deliberately leaves ordinary values unchanged for compatibility:
// "community" remains "community", while "a_b" becomes "a%5Fb". The percent
// sign is escaped first so the encoding is injective, and "~" is reserved for
// a missing optional dimension.
func encodeDimensionValue(v string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		"_", "%5F",
		"~", "%7E",
	)
	return replacer.Replace(v)
}
