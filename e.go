package goatcounter

import (
	"time"

	"zgo.at/zstd/zbool"
)

type (
	ExportInfo struct {
		ExportVersion      string    `json:"export_version"`
		GoatcounterVersion string    `json:"goatcounter_version"`
		CreatedFor         string    `json:"created_for"`
		CreatedBy          string    `json:"created_by"`
		CreatedAt          time.Time `json:"created_at"`
	}
	ExportPath struct {
		ID    PathID     `json:"id"`
		Path  string     `json:"path"`
		Title string     `json:"title"`
		Event zbool.Bool `json:"event,omitempty"`
	}
	ExportLanguage struct {
		ISO6393 string `db:"iso_639_3" json:"iso_639_3"`
		Name    string `db:"name" json:"name"`
	}
	ExportBrowserStat struct {
		Day       string    `db:"day" json:"day"`
		PathID    PathID    `db:"path_id" json:"path_id"`
		BrowserID BrowserID `db:"browser_id" json:"browser_id"`
		Count     int       `db:"count" json:"count"`
	}
	ExportSystemStat struct {
		Day      string   `db:"day" json:"day"`
		PathID   PathID   `db:"path_id" json:"path_id"`
		SystemID SystemID `db:"system_id" json:"system_id"`
		Count    int      `db:"count" json:"count"`
	}
	ExportLocationStat struct {
		Day      string `db:"day" json:"day"`
		PathID   PathID `db:"path_id" json:"path_id"`
		Location string `db:"location" json:"location"`
		Count    int    `db:"count" json:"count"`
	}
	ExportSizeStat struct {
		Day    string `db:"day" json:"day"`
		PathID PathID `db:"path_id" json:"path_id"`
		Width  int    `db:"width" json:"width"`
		Count  int    `db:"count" json:"count"`
	}
	ExportLanguageStat struct {
		Day      string `db:"day" json:"day"`
		PathID   PathID `db:"path_id" json:"path_id"`
		Language string `db:"language" json:"language"`
		Count    int    `db:"count" json:"count"`
	}
	ExportCampaignStat struct {
		Day        string     `db:"day" json:"day"`
		PathID     PathID     `db:"path_id" json:"path_id"`
		CampaignID CampaignID `db:"campaign_id" json:"campaign_id"`
		Ref        string     `db:"ref" json:"ref"`
		Count      int        `db:"count" json:"count"`
	}
	ExportHitStat struct {
		Hour   string `db:"hour" json:"hour"`
		PathID PathID `db:"path_id" json:"path_id"`
		RefID  RefID  `db:"ref_id" json:"ref_id"`
		Count  int    `db:"total" json:"count"`
	}
)
