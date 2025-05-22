package gomig

import "context"

var Migrations = map[string]func(context.Context) error{
	"2021-12-08-1-set-chart-text":    KeepAsText,
	"2022-11-15-1-correct-hit-stats": CorrectHitStats,
	"2025-07-01-1-share-api-tokens":  ShareAPITokens,
}
