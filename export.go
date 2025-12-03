package goatcounter

import (
	"context"
	"os"
	"time"

	"zgo.at/errors"
	"zgo.at/zdb"
)

type ExportID int32

type Export struct {
	ID     ExportID `db:"export_id" json:"id,readonly"`
	SiteID SiteID   `db:"site_id" json:"site_id,readonly"`

	// The hit ID this export was started from.
	StartFromHitID HitID `db:"start_from_hit_id" json:"start_from_hit_id"`

	// Last hit ID that was exported; can be used as start_from_hit_id.
	LastHitID *HitID `db:"last_hit_id" json:"last_hit_id,readonly"`

	Path      string    `db:"path" json:"path,readonly"` // {omitdoc}
	CreatedAt time.Time `db:"created_at" json:"created_at,readonly"`

	FinishedAt *time.Time `db:"finished_at" json:"finished_at,readonly"`
	NumRows    *int       `db:"num_rows" json:"num_rows,readonly"`

	// File size in MB.
	Size *string `db:"size" json:"size,readonly"`

	// SHA256 hash.
	Hash *string `db:"hash" json:"hash,readonly"`

	// Any errors that may have occured.
	Error *string `db:"error" json:"error,readonly"`
}

func (e *Export) ByID(ctx context.Context, id ExportID) error {
	err := zdb.Get(ctx, e, `/* Export.ByID */
		select * from exports where export_id=$1 and site_id=$2`,
		id, MustGetSite(ctx).ID)
	return errors.Wrapf(err, "Export.ByID(%d)", id)
}

// Exists reports whether this export exists on the filesystem.
func (e Export) Exists() bool {
	if e.Path == "" {
		return false
	}
	_, err := os.Stat(e.Path)
	return err == nil
}

type Exports []Export

func (e *Exports) List(ctx context.Context) error {
	err := zdb.Select(ctx, e, `/* Exports.List */
		select * from exports where site_id=$1 order by created_at desc limit 10`,
		MustGetSite(ctx).ID)
	return errors.Wrap(err, "Exports.List")
}
