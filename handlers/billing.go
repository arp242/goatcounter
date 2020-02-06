// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi"
	"github.com/teamwork/guru"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cfg"
	"zgo.at/utils/jsonutil"
	"zgo.at/zdb"
	"zgo.at/zhttp"
	"zgo.at/zlog"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

type billing struct{}

func (h billing) mount(pub, auth chi.Router) {
	auth.Get("/billing", zhttp.Wrap(h.index))

	pauth := auth.With(noSubSites)
	pauth.Post("/billing/start", zhttp.Wrap(h.start))
	pauth.Get("/billing/cancel", zhttp.Wrap(h.confirmCancel))
	pauth.Post("/billing/cancel", zhttp.Wrap(h.cancel))

	// These are not specific to any domain.
	pub.Post("/stripe-webhook", zhttp.Wrap(h.stripeWebhook))
}

func (h billing) index(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())

	switch r.URL.Query().Get("return") {
	case "cancel":
		zhttp.FlashError(w, "Payment cancelled.")

	case "success":
		// Verify that the webhook was processed correct.
		if site.Stripe == nil || site.UpdatedAt.Before(time.Now().UTC().Add(-1*time.Minute)) {
			zhttp.Flash(w, "The payment processor reported success, but we're still processing the payment")
			stripe := ""
			if site.Stripe != nil {
				stripe = *site.Stripe
			}
			zlog.Fields(zlog.F{
				"siteID":   site.ID,
				"stripeID": stripe,
			}).Errorf("stripe not processed")
		} else {
			zhttp.Flash(w, "Payment processed successfully!")
		}
	}

	var payment, next string
	if site.Stripe != nil && !site.FreePlan() {
		var customer struct {
			Subscriptions struct {
				Data []struct {
					CancelAtPeriodEnd bool               `json:"cancel_at_period_end"`
					CurrentPeriodEnd  jsonutil.Timestamp `json:"current_period_end"`
					Plan              struct {
						Quantity int `json:"quantity"`
					} `json:"plan"`
				} `json:"data"`
			} `json:"subscriptions"`
		}
		_, err := zstripe.Request(&customer, "GET",
			fmt.Sprintf("/v1/customers/%s", *site.Stripe), "")
		if err != nil {
			return err
		}

		if len(customer.Subscriptions.Data) > 0 {
			var methods struct {
				Data []struct {
					Card struct {
						Brand string `json:"brand"`
						Last4 string `json:"last4"`
					} `json:"card"`
				} `json:"data"`
			}
			_, err = zstripe.Request(&methods, "GET", "/v1/payment_methods", zstripe.Body{
				"customer": *site.Stripe,
				"type":     "card",
			}.Encode())
			if err != nil {
				return err
			}
			if len(methods.Data) > 0 {
				payment = fmt.Sprintf("%s card ending with %s",
					methods.Data[0].Card.Brand, methods.Data[0].Card.Last4)
			}

			var invoice struct {
				AmountDue int                `json:"amount_due"`
				Created   jsonutil.Timestamp `json:"created"`
			}
			_, err = zstripe.Request(&invoice, "GET", "/v1/invoices/upcoming", zstripe.Body{
				"customer": *site.Stripe,
			}.Encode())
			if err != nil {
				return err
			}
			next = fmt.Sprintf("Next invoice will be for €%d on %s.",
				invoice.AmountDue/100, invoice.Created.Format("Jan 2, 2006"))
		}
	}

	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
		StripePublicKey string
		Payment         string
		Next            string
		Subscribed      bool
		FreePlan        bool
	}{newGlobals(w, r), zstripe.PublicKey, payment, next,
		payment != "", site.FreePlan()})
}

var stripePlans = map[bool]map[string]string{
	true: { // Production
		"personal":     "plan_GLVKIvCvCjzT2u",
		"personalplus": "plan_GlJxixkxNZZOct",
		"business":     "plan_GLVGCVzLaPA3cY",
		"businessplus": "plan_GLVHJUi21iV4Wh",
	},
	false: { // Test data
		"personal":     "plan_GLWnXaogEns1n2",
		"personalplus": "plan_GlJvCPdCeUpww3",
		"business":     "plan_GLWoJ72fcNGoUD",
		"businessplus": "plan_GLWootweDZnKBk",
	},
}

func (h billing) start(w http.ResponseWriter, r *http.Request) error {
	site := goatcounter.MustGetSite(r.Context())

	var args struct {
		Plan     string `json:"plan"`
		Quantity string `json:"quantity"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	v := zvalidate.New()
	v.Required("plan", args.Plan)
	v.Include("plan", args.Plan, goatcounter.Plans)
	v.Required("quantity", args.Quantity)
	quantity := v.Integer("quantity", args.Quantity)
	if v.HasErrors() {
		return v
	}

	// Use dummy Stripe customer for personal plan without donations.
	if args.Plan == goatcounter.PlanPersonal && quantity == 0 {
		err := site.UpdateStripe(r.Context(),
			fmt.Sprintf("cus_free_%d", site.ID),
			goatcounter.PlanPersonal)
		if err != nil {
			return err
		}
		zhttp.Flash(w, "Saved!")
		return zhttp.JSON(w, "")
	}

	body := zstripe.Body{
		"mode":                                 "subscription",
		"payment_method_types[]":               "card",
		"client_reference_id":                  strconv.FormatInt(site.ID, 10),
		"success_url":                          site.URL() + "/billing?return=success",
		"cancel_url":                           site.URL() + "/billing?return=cancel",
		"subscription_data[items][][plan]":     stripePlans[cfg.Prod][args.Plan],
		"subscription_data[items][][quantity]": args.Quantity,
	}
	if site.Stripe != nil && !site.FreePlan() {
		body["customer"] = *site.Stripe
	} else {
		body["customer_email"] = goatcounter.GetUser(r.Context()).Email
	}

	var id zstripe.ID
	_, err = zstripe.Request(&id, "POST", "/v1/checkout/sessions", body.Encode())
	if err != nil {
		return err
	}

	return zhttp.JSON(w, id)
}

func (h billing) cancel(w http.ResponseWriter, r *http.Request) error {
	var customer struct {
		Subscriptions struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"subscriptions"`
	}

	site := goatcounter.MustGetSite(r.Context())
	if site.Stripe == nil {
		return guru.New(400, "No Stripe customer for this site?")
	}

	zdb.TX(r.Context(), func(ctx context.Context, db zdb.DB) error {
		err := site.UpdateStripe(r.Context(), *site.Stripe, goatcounter.PlanPersonal)
		if err != nil {
			return err
		}

		_, err = zstripe.Request(&customer, "GET",
			fmt.Sprintf("/v1/customers/%s", *site.Stripe), "")
		if err != nil {
			return err
		}

		if len(customer.Subscriptions.Data) == 0 {
			zhttp.FlashError(w, "No current subscriptions")
			return zhttp.SeeOther(w, "/billing")
		}
		if len(customer.Subscriptions.Data) > 1 {
			return fmt.Errorf(
				"billing.cancel: unexpected number of subscriptions for site %d/%s",
				site.ID, *site.Stripe)
		}

		_, err = zstripe.Request(nil, "DELETE",
			fmt.Sprintf("/v1/subscriptions/%s", customer.Subscriptions.Data[0].ID),
			zstripe.Body{"prorate": "true"}.Encode())
		return err
	})

	zhttp.Flash(w, "Plan cancelled; you will be refunded for the remaining period.")
	return zhttp.SeeOther(w, "/billing")
}

type Session struct {
	ClientReferenceID string `json:"client_reference_id"`
	Customer          string `json:"customer"`
	DisplayItems      []struct {
		Plan struct {
			Nickname string `json:"nickname"`
		} `json:"plan"`
	} `json:"display_items"`
}

// Test webhooks with the Stripe CLI:
//   ./stripe listen  --forward-to 'arp242.goatcounter.localhost:8081/stripe-webhook'
func (h billing) stripeWebhook(w http.ResponseWriter, r *http.Request) error {
	var event zstripe.Event
	err := event.Read(r)
	if err != nil {
		return err
	}

	switch event.Type {
	case "checkout.session.completed":
		var s Session
		err := json.Unmarshal(event.Data.Raw, &s)
		if err != nil {
			return err
		}

		go func(r *http.Request, s Session) {
			ctx := zdb.With(context.Background(), zdb.MustGet(r.Context()))

			l := zlog.Module("stripe-wh").FieldsRequest(r).Field("session", s)
			id, err := strconv.ParseInt(s.ClientReferenceID, 10, 64)
			if err != nil {
				l.Error(err)
				return
			}

			var site goatcounter.Site
			err = site.ByID(ctx, id)
			if err != nil {
				l.Error(err)
				return
			}

			err = site.UpdateStripe(ctx, s.Customer, s.DisplayItems[0].Plan.Nickname)
			if err != nil {
				l.Error(err)
				return
			}
		}(r, s)
	}

	return zhttp.String(w, "okay")
}

func (h billing) confirmCancel(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "billing_cancel.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}
