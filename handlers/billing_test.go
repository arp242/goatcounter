// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package handlers

import (
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zdb"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztest"
	"zgo.at/zstd/ztime"
	"zgo.at/zstripe"
)

func TestSettingsBilling(t *testing.T) {
	tp := func(t time.Time) *time.Time { return &t }
	ztime.SetNow(t, "2020-06-20")
	tests := []struct {
		name string
		site *goatcounter.Site
		want []string
	}{
		{
			"trial",
			&goatcounter.Site{
				Plan: goatcounter.PlanTrial,
			},
			[]string{
				"Currently using the Free plan; the limits for this are",
				"Your billing period starts at the 1st of every month",
				"Subscribe to a plan",

				"!Your next invoice",
				"!Manage subscription",
			},
		},
		{
			"free plan",
			&goatcounter.Site{
				Plan: goatcounter.PlanFree,
			},
			[]string{
				"Currently using the Free plan; the limits for this are",
				"Your billing period starts at the 1st of every month",
				"Subscribe to a plan",

				"!Your next invoice",
				"!Manage subscription",
			},
		},
		{
			"personal",
			&goatcounter.Site{
				Plan:          goatcounter.PlanPersonal,
				Stripe:        zstring.NewPtr("cus_asd").P,
				BillingAmount: zstring.NewPtr("EUR 2").P,
				BillingAnchor: tp(ztime.New("2020-06-18")),
			},
			[]string{
				"Currently using the Personal plan; the limits for this are",
				"Your billing period starts at the 18th of every month",
				"Your next invoice will be on Jul 18th",
				"Manage subscription",

				"!Subscribe to a plan",
			},
		},
		{
			"starter subscription",
			&goatcounter.Site{
				Plan:          goatcounter.PlanStarter,
				Stripe:        zstring.NewPtr("cus_abc").P,
				BillingAmount: zstring.NewPtr("EUR 5").P,
				BillingAnchor: tp(ztime.New("2020-06-18")),
			},
			[]string{
				"Currently using the Starter plan; the limits for this are",
				"Your billing period starts at the 18th of every month",
				"Your next invoice will be on Jul 18th",
				"Manage subscription",

				"!Subscribe to a plan",
			},
		},

		{
			"scheduled cancel",
			&goatcounter.Site{
				Plan:          goatcounter.PlanStarter,
				Stripe:        zstring.NewPtr("cus_abc").P,
				BillingAmount: zstring.NewPtr("EUR 5").P,
				BillingAnchor: tp(ztime.New("2020-06-18")),
				PlanCancelAt:  tp(ztime.New("2020-07-18")),
			},
			[]string{
				"Currently using the Starter plan; the limits for this are",
				"Your billing period starts at the 18th of every month",
				"Manage subscription",
				"scheduled to be cancelled on Jul 18th",

				"!Subscribe to a plan",
				"!next invoice",
			},
		},

		{
			"cancelled",
			&goatcounter.Site{
				Stripe: zstring.NewPtr("cus_abc").P,
			},
			[]string{
				"Currently using the Free plan; the limits for this are",
				"Your billing period starts at the 1st of every month",
				"Subscribe to a plan",

				"!Your next invoice",
				"!Manage subscription",
			},
		},
	}

	zstripe.SecretKey, zstripe.SignSecret, zstripe.PublicKey = "x", "x", "x"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := gctest.DB(t)
			goatcounter.Config(ctx).GoatcounterCom = true
			ctx = gctest.Site(ctx, t, tt.site, nil)

			r, rr := newLoginTest(t, ctx, "GET", "/billing", nil)
			newBackend(zdb.MustGetDB(ctx)).ServeHTTP(rr, r)
			ztest.Code(t, rr, 200)

			have := bodyText(t, rr)
			for _, w := range tt.want {
				if !matchText(have, w) {
					t.Error(w)
				}
			}
			if t.Failed() {
				t.Log("\n" + have)
			}
		})
	}
}

func bodyHtml(t *testing.T, rr *httptest.ResponseRecorder) string {
	doc, err := goquery.NewDocumentFromReader(rr.Body)
	if err != nil {
		t.Fatal(err)
	}

	sel := doc.Find(".page")
	sel.Find("nav.tab-nav").Remove() // Settings nav

	b, err := sel.Html()
	if err != nil {
		t.Fatal(err)
	}
	return regexp.MustCompile(`\n\s*\n+`).ReplaceAllString(strings.TrimSpace(b), "\n")
}

func bodyText(t *testing.T, rr *httptest.ResponseRecorder) string {
	doc, err := goquery.NewDocumentFromReader(rr.Body)
	if err != nil {
		t.Fatal(err)
	}

	sel := doc.Find(".page")
	sel.Find("nav.tab-nav").Remove() // Settings nav

	return regexp.MustCompile(`\n\s*\n+`).ReplaceAllString(strings.TrimSpace(sel.Text()), "\n")
}

func matchText(body, find string) bool {
	not := strings.HasPrefix(find, "!")
	if not {
		find = find[1:]
	}

	p := strings.ReplaceAll(regexp.QuoteMeta(strings.ToLower(find)), " ", `\s+`)
	m := regexp.MustCompile(p).MatchString(strings.ToLower(body))
	if not {
		m = !m
	}
	return m
}
