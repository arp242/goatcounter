// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter"
	"zgo.at/zhttp"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

type billing struct{}

func (h billing) mount(r chi.Router) {
	r.Get("/billing", zhttp.Wrap(h.index))          // Overview of status, invoices
	r.Post("/billing/start", zhttp.Wrap(h.start))   // create Stripe plan
	r.Post("/billing/stop", zhttp.Wrap(h.stop))     // Cancel
	r.Post("/billing/change", zhttp.Wrap(h.change)) // Change plan, CC, etc.
}

type BillingInfo struct {
	Plan    string
	Payment string
}

func (h billing) index(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())

	info := BillingInfo{Plan: site.PlanName(r.Context())}
	if site.Stripe != nil {
		var methods struct {
			Data []struct {
				Card struct {
					Brand string `json:"brand"`
					Last4 string `json:"last4"`
				} `json:"card"`
			} `json:"data"`
		}
		_, err := zstripe.Request(&methods, "GET", "/v1/payment_methods", zstripe.Body{
			"customer": *site.Stripe,
			"type":     "card",
		}.Encode())
		if err != nil {
			return err
		}

		info.Payment = "No payment details on record"
		if len(methods.Data) > 0 {
			info.Payment = fmt.Sprintf("%s card ending with %s",
				methods.Data[0].Card.Brand, methods.Data[0].Card.Last4)
		}
	}

	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
		Info BillingInfo
	}{newGlobals(w, r), info})
}

type Card struct {
	Number   string `json:"number"`
	ExpMonth string `json:"exp_month"`
	ExpYear  string `json:"exp_year"`
	CVC      string `json:"cvc"`
}

func (h billing) start(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		Number   string `json:"number"`
		Optional string `json:"optional"`
		Expiry   string `json:"exp_month"`
		CVC      string `json:"cvc"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	v := zvalidate.New()
	v.Required("number", args.Number)
	v.Required("expiry", args.Expiry)
	v.Required("cvc", args.CVC)
	// TODO
	//args.Number, _ = v.CreditCard("number", args.Number)
	v.Len("cvc", args.CVC, 3, 3)

	card := Card{Number: args.Number, CVC: args.CVC}
	p := strings.IndexAny(args.Expiry, "/- ")
	if p == -1 {
		return guru.Errorf(400, "invalid expiry: %q", args.Expiry)
	}
	card.ExpMonth = args.Expiry[:p]
	card.ExpYear = args.Expiry[p+1:]
	if len(card.ExpYear) == 2 {
		card.ExpYear = "20" + card.ExpYear
	}

	if v.HasErrors() {
		return v
	}

	site := goatcounter.MustGetSite(r.Context())
	user := goatcounter.GetUser(r.Context())
	if user == nil {
		return fmt.Errorf("no user on context?!")
	}

	// Create customer and payment method if there isn't one yet.
	if site.Stripe == nil {
		// Create payment method; this will validate the card.
		var p zstripe.ID
		_, err = zstripe.Request(&p, "POST", "/v1/payment_methods", zstripe.Body{
			"type":            "card",
			"card[number]":    card.Number,
			"card[exp_month]": card.ExpMonth,
			"card[exp_year]":  card.ExpYear,
			"card[cvc]":       card.CVC,
		}.Encode())
		if err != nil {
			return err
		}

		// Create customer.
		var c zstripe.ID
		_, err := zstripe.Request(&c, "POST", "/v1/customers", zstripe.Body{
			"name":           user.Name,
			"email":          user.Email,
			"description":    fmt.Sprintf("%s (site %d)", site.Name, site.ID),
			"payment_method": p.ID,
			"invoice_settings[default_payment_method]": p.ID,
		}.Encode())
		if err != nil {
			return err
		}

		site.Stripe = &c.ID
		err = site.UpdateStripe(r.Context())
		if err != nil {
			return err
		}
	}

	// Create subscription.
	body := zstripe.Body{
		"customer":          *site.Stripe,
		"collection_method": "charge_automatically",
	}

	start := site.CreatedAt.Add(24 * time.Hour * 14).UTC()
	now := time.Now().UTC()
	if start.Before(now) {
		start = now
	}
	body["billing_cycle_anchor"] = fmt.Sprintf("%d", start.Unix())
	body["items[][plan]"] = map[string]string{
		"b": "plan_GBM4KASBqV3nvG",
		"p": "plan_GKL0nIQtDkcpr0",
		"f": "plan_GKLnCYIsnVHREH",
	}[site.Plan]
	if site.Plan == "p" {
		body["items[][quantity]"] = "1" // TODO: fill correct
	}

	var s zstripe.ID
	_, err = zstripe.Request(&s, "POST", "/v1/subscriptions", body.Encode())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}

func (h billing) stop(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}

func (h billing) change(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}
