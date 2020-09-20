// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package goatcounter

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"zgo.at/errors"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstd/zjson"
)

type AdminStat struct {
	ID            int64     `db:"site_id"`
	Parent        *int64    `db:"parent"`
	Code          string    `db:"code"`
	Stripe        *string   `db:"stripe"`
	BillingAmount *string   `db:"billing_amount"`
	LinkDomain    string    `db:"link_domain"`
	Email         string    `db:"email"`
	CreatedAt     time.Time `db:"created_at"`
	Plan          string    `db:"plan"`
	LastMonth     int       `db:"last_month"`
	Total         int       `db:"total"`
}

type AdminStats []AdminStat

// List stats for all sites, for all time.
func (a *AdminStats) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, a, fmt.Sprintf(`/* AdminStats.List */
		select
			sites.site_id,
			sites.parent,
			sites.code,
			sites.created_at,
			sites.billing_amount,
			(case
				when sites.stripe is null then 'free'
				when substr(sites.stripe, 0, 9) = 'cus_free' then 'free'
				else sites.plan
			end) as plan,
			stripe,
			sites.link_domain,
			(select email from users where site_id=sites.site_id or site_id=sites.parent) as email,
			coalesce((
				select sum(hit_counts.total) from hit_counts where site_id=sites.site_id
			), 0) as total,
			coalesce((
				select sum(hit_counts.total) from hit_counts
				where site_id=sites.site_id and hit_counts.hour >= %s
			), 0) as last_month
		from sites
		order by last_month desc`, interval(30)))
	if err != nil {
		return errors.Wrap(err, "AdminStats.List")
	}

	// Add all the child plan counts to the parents.
	aa := *a
	for _, s := range aa {
		if s.Plan != PlanChild {
			continue
		}

		for i, s2 := range aa {
			if s2.ID == *s.Parent {
				aa[i].Total += s.Total
				aa[i].LastMonth += s.LastMonth
				break
			}
		}
	}
	sort.Slice(aa, func(i, j int) bool { return aa[i].LastMonth > aa[j].LastMonth })
	return nil
}

type AdminSiteStat struct {
	Site           Site      `db:"-"`
	User           User      `db:"-"`
	LastData       time.Time `db:"last_data"`
	CountTotal     int       `db:"count_total"`
	CountLastMonth int       `db:"count_last_month"`
	CountPrevMonth int       `db:"count_prev_month"`
}

// ByID gets stats for a single site.
func (a *AdminSiteStat) ByID(ctx context.Context, id int64) error {
	err := a.Site.ByID(ctx, id)
	if err != nil {
		return err
	}

	err = a.User.BySite(ctx, id)
	if err != nil {
		return err
	}

	ival30 := interval(30)
	ival60 := interval(30)
	err = zdb.MustGet(ctx).GetContext(ctx, a, fmt.Sprintf(`/* *AdminSiteStat.ByID */
		select
			coalesce((select hour from hit_counts where site_id=$1 order by hour desc limit 1), '1970-01-01') as last_data,
			coalesce((select sum(total) from hit_counts where site_id=$1), 0) as count_total,
			coalesce((select sum(total) from hit_counts where site_id=$1
				and hour >= %[1]s), 0) as count_last_month,
			coalesce((select sum(total) from hit_counts where site_id=$1
				and hour >= %[2]s
				and hour <= %[1]s
			), 0) as count_prev_month
		`, ival30, ival60), id)
	return errors.Wrap(err, "AdminSiteStats.ByID")
}

// ByCode gets stats for a single site.
func (a *AdminSiteStat) ByCode(ctx context.Context, code string) error {
	err := a.Site.ByHost(ctx, code+"."+cfg.Domain)
	if err != nil {
		return err
	}
	return a.ByID(ctx, a.Site.ID)
}

type AdminBotlog struct {
	ID int64 `json:"id"`
	IP int64 `json:"ip"`

	Bot       int         `db:"bot"`
	UserAgent string      `db:"ua"`
	Headers   http.Header `db:"headers"`
	URL       string      `db:"url"`
}

type AdminBotlogIP struct {
	ID        int64     `db:"id"`
	Count     int       `db:"count"`
	IP        string    `db:"ip"`
	PTR       *string   `db:"ptr"`
	Info      *string   `db:"info"`
	Hide      zdb.Bool  `db:"hide"`
	CreatedAt time.Time `db:"created_at"`
	LastSeen  time.Time `db:"last_seen"`

	Links [][]string `db:"-"`
}

type AdminBotlogIPs []AdminBotlogIP

func (b *AdminBotlogIPs) List(ctx context.Context) error {
	err := zdb.MustGet(ctx).SelectContext(ctx, b,
		`select * from botlog_ips where hide=0 order by count desc limit 100`)
	if err != nil {
		return errors.Wrap(err, "AdminBotlogIps.List")
	}

	bb := *b
	for i := range bb {
		if bb[i].Info == nil {
			continue
		}
		t := strings.TrimSpace(*bb[i].Info)
		bb[i].Info = &t
		for _, line := range strings.Split(*bb[i].Info, "\n") {
			if strings.HasPrefix(line, "org:") || strings.HasPrefix(line, "mnt-by:") || strings.HasPrefix(line, "netname:") {
				bb[i].Links = append(bb[i].Links, strings.Fields(line))
			}
		}
	}

	return nil
}

func (b AdminBotlog) Insert(ctx context.Context, ip string) error {
	ip = zhttp.RemovePort(ip)

	txctx, tx, err := zdb.Begin(ctx)
	if err != nil {
		return errors.Errorf("AdminBotlog.Insert Begin: %w", err)
	}
	defer tx.Rollback()

	var (
		ipID  int
		newIP bool
	)
	err = tx.GetContext(txctx, &ipID, `select id from botlog_ips where ip=$1`, ip)
	if err != nil && !zdb.ErrNoRows(err) {
		return errors.Errorf("AdminBotlog.Insert get IP: %w", err)
	}

	if ipID == 0 {
		newIP = true
		err := tx.GetContext(txctx, &ipID, `insert into botlog_ips
			(ip, created_at, last_seen) values ($1, now(), now())
			on conflict on constraint "botlog_ips#ip" do update
				set last_seen=now(), count = botlog_ips.count + 1
			returning id`, ip)
		if err != nil {
			return errors.Errorf("AdminBotlog.Insert insert ip: %w", err)
		}
	} else {
		_, err := tx.ExecContext(txctx,
			`update botlog_ips set count=count+1, last_seen=now() where id=$1`,
			ipID)
		if err != nil {
			return errors.Errorf("AdminBotlog.Insert update count: %w", err)
		}
	}

	_, err = tx.ExecContext(txctx, `insert into botlog
			(ip, bot, ua, headers, url, created_at) values ($1, $2, $3, $4, $5, now())`,
		ipID, b.Bot, b.UserAgent, zjson.MustMarshal(b.Headers), b.URL)
	if err != nil {
		return errors.Errorf("AdminBotlog.Insert botlog: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return errors.Errorf("AdminBotlog.Insert commit: %w", err)
	}

	if newIP {
		bgrun.Run("botlog", func() {
			// apk add whois drill
			whois, _ := exec.Command("whois", "-r", "--", "--resource", ip).CombinedOutput()
			var info strings.Builder
			if err == nil {
				for _, line := range bytes.Split(bytes.TrimSpace(whois), []byte("\n")) {
					if len(line) == 0 {
						info.WriteRune('\n')
						continue
					}

					if line[0] == '%' ||
						bytes.HasPrefix(line, []byte("remarks:")) ||
						bytes.HasPrefix(line, []byte("created:")) ||
						bytes.HasPrefix(line, []byte("last-modified:")) {
						continue
					}
				}
			}

			drill, err := exec.Command("drill", "-x", ip).CombinedOutput()
			var ptr string
			if err == nil {
				lines := bytes.Split(drill, []byte("\n"))
				for i, line := range lines {
					if bytes.HasPrefix(line, []byte(";; ANSWER SECTION:")) {
						answer := string(lines[i+1])
						if !strings.Contains(answer, "PTR") {
							ptr = "<not set>"
						} else {
							x := strings.Fields(answer)
							ptr = x[len(x)-1]
						}
						break
					}
				}
			}

			_, err = zdb.MustGet(ctx).ExecContext(ctx,
				`update botlog_ips set ptr=$1, info=$2 where id=$3`,
				ptr, strings.TrimSpace(info.String()), ipID)
			if err != nil {
				zlog.Errorf("AdminBotlog.Insert: %w", err)
			}
		})
	}

	return nil
}
