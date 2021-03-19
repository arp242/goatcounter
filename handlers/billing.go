// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

/* Test webhooks with the Stripe CLI:

./stripe listen -sjl -f 'arp242.goatcounter.localhost:8081/stripe-webhook' \
	-e checkout.session.completed,customer.subscription.updated,customer.subscription.deleted
*/

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"zgo.at/blackmail"
	"zgo.at/errors"
	"zgo.at/goatcounter"
	"zgo.at/goatcounter/bgrun"
	"zgo.at/guru"
	"zgo.at/json"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zlog"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zstring"
	"zgo.at/zstripe"
	"zgo.at/zvalidate"
)

var stripePlans = map[bool]map[string]string{
	false: { // Production
		"personal":     "plan_GLVKIvCvCjzT2u",
		"personalplus": "plan_GlJxixkxNZZOct",
		"business":     "plan_GLVGCVzLaPA3cY",
		"businessplus": "plan_GLVHJUi21iV4Wh",
		"donate":       "sku_H9jE6zFGzh6KKb",
	},
	true: { // Test data
		"personal":     "price_1IVVd4E5ASM7XVaUGld6Fwys",
		"personalplus": "price_1IVVbtE5ASM7XVaUABPNZewg",
		"business":     "price_1IVVh1E5ASM7XVaUK3q5leCi",
		"businessplus": "price_1IVVg6E5ASM7XVaUZrJwlrfn",
		"donate":       "sku_J7l6316cimcIC5",
	},
}

type billing struct{}

func (h billing) mount(pub, auth chi.Router) {
	// Already added in backend.
	if !zstring.Contains(zlog.Config.Debug, "req") && !zstring.Contains(zlog.Config.Debug, "all") {
		pub = pub.With(mware.RequestLog(nil))
		auth = auth.With(mware.RequestLog(nil))
	}
	// Not specific to any domain, and works on any (valid) domain.
	pub.Post("/stripe-webhook", zhttp.Wrap(h.webhook))

	auth.Get("/billing", zhttp.Wrap(h.index))
	auth.Post("/billing/manage", zhttp.Wrap(h.manage))
	auth.Post("/billing/start", zhttp.Wrap(h.start))
}

func (h billing) index(w http.ResponseWriter, r *http.Request) error {
	mainSite, err := MainSite(r.Context())
	if err != nil {
		return err
	}

	switch r.URL.Query().Get("return") {
	case "cancel":
		zhttp.FlashError(w, "Payment cancelled.")

	case "success":
		// Verify that the webhook was processed correct.
		if mainSite.Stripe == nil {
			zhttp.Flash(w, "The payment processor reported success, but we're still processing the payment")
			stripe := ""
			if mainSite.Stripe != nil {
				stripe = *mainSite.Stripe
			}
			zlog.Fields(zlog.F{
				"siteID":   mainSite.ID,
				"stripeID": stripe,
			}).Errorf("stripe not processed")
		} else {
			bgrun.Run("email:subscription", func() {
				blackmail.Send("New GoatCounter subscription "+mainSite.Plan,
					blackmail.From("GoatCounter Billing", "billing@goatcounter.com"),
					blackmail.To("billing@goatcounter.com"),
					blackmail.Bodyf(`New subscription: %s (%d) %s`, mainSite.Code, mainSite.ID, *mainSite.Stripe))
			})
			zhttp.Flash(w, "Payment processed successfully!")
		}
	}

	external := mainSite.PayExternal()
	var payment, next, cancel string
	if external != "" {
		payment = external
	}

	// Load current data from the Stripe API.
	if mainSite.Stripe != nil && !mainSite.FreePlan() && external == "" {
		payment = "[Error loading data from Stripe]"

		var customer struct {
			Subscriptions struct {
				Data []struct {
					CancelAt zjson.Timestamp `json:"cancel_at"`
					Plan     struct {
						Quantity int `json:"quantity"`
					} `json:"plan"`
				} `json:"data"`
			} `json:"subscriptions"`
		}
		_, err := zstripe.Request(&customer, "GET",
			fmt.Sprintf("/v1/customers/%s", *mainSite.Stripe), zstripe.Body{
				"expand[]": "subscriptions",
			}.Encode())
		if err != nil {
			return err
		}

		if len(customer.Subscriptions.Data) > 0 {
			if !customer.Subscriptions.Data[0].CancelAt.IsZero() {
				cancel = customer.Subscriptions.Data[0].CancelAt.Format("Jan 2, 2006")
			}

			var methods struct {
				Data []struct {
					Card struct {
						Brand string `json:"brand"`
						Last4 string `json:"last4"`
					} `json:"card"`
				} `json:"data"`
			}
			_, err = zstripe.Request(&methods, "GET", "/v1/payment_methods", zstripe.Body{
				"customer": *mainSite.Stripe,
				"type":     "card",
			}.Encode())
			if err != nil {
				return err
			}
			if len(methods.Data) > 0 {
				payment = fmt.Sprintf("a %s card ending with %s",
					methods.Data[0].Card.Brand, methods.Data[0].Card.Last4)
			}

			var invoice struct {
				AmountDue int             `json:"amount_due"`
				Created   zjson.Timestamp `json:"created"`
			}
			_, err = zstripe.Request(&invoice, "GET", "/v1/invoices/upcoming", zstripe.Body{
				"customer": *mainSite.Stripe,
			}.Encode())
			if err != nil {
				return err
			}
			next = fmt.Sprintf("Next invoice will be for €%d on %s.",
				invoice.AmountDue/100, invoice.Created.Format("Jan 2, 2006"))
		}
	}

	var sites goatcounter.Sites
	err = sites.ForThisAccount(r.Context(), false)
	if err != nil {
		return err
	}

	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
		MainSite        *goatcounter.Site
		Sites           goatcounter.Sites
		StripePublicKey string
		Payment         string
		Next            string
		Cancel          string
		Subscribed      bool
		External        string
	}{newGlobals(w, r), mainSite, sites, zstripe.PublicKey, payment, next,
		cancel, payment != "", external})
}

func (h billing) start(w http.ResponseWriter, r *http.Request) error {
	mainSite := Site(r.Context())
	err := mainSite.GetMain(r.Context())
	if err != nil {
		return err
	}

	var args struct {
		Plan     string `json:"plan"`
		Quantity string `json:"quantity"`
		NoDonate string `json:"nodonate"`
	}
	_, err = zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	// Temporary log, since I got some JS "Bad Request" errors without any
	// detail which I can't reproduce :-/
	zlog.Fields(zlog.F{
		"args": args,
		"site": mainSite.Code,
	}).Printf("billing/start")

	v := zvalidate.New()
	v.Required("plan", args.Plan)
	v.Include("plan", args.Plan, goatcounter.Plans)
	v.Required("quantity", args.Quantity)
	v.Integer("quantity", args.Quantity)
	if v.HasErrors() {
		return v
	}

	// Use dummy Stripe customer for personal plan without donations; don't need
	// to send anything to Stripe.
	if args.Plan == goatcounter.PlanPersonal && args.NoDonate == "true" {
		mainSite.Stripe = zstring.NewPtr(fmt.Sprintf("cus_free_%d", mainSite.ID)).P
		mainSite.BillingAmount = nil
		mainSite.Plan = goatcounter.PlanPersonal
		mainSite.PlanPending = nil
		mainSite.PlanCancelAt = nil
		err := mainSite.UpdateStripe(r.Context())
		if err != nil {
			return err
		}
		zhttp.Flash(w, "Saved!")
		return zhttp.JSON(w, `{"status":"ok","no_stripe":true}`)
	}

	body := zstripe.Body{
		"mode":                                 "subscription",
		"payment_method_types[]":               "card",
		"client_reference_id":                  strconv.FormatInt(mainSite.ID, 10),
		"success_url":                          mainSite.URL(r.Context()) + "/billing?return=success",
		"cancel_url":                           mainSite.URL(r.Context()) + "/billing?return=cancel",
		"subscription_data[items][][plan]":     stripePlans[goatcounter.Config(r.Context()).Dev][args.Plan],
		"subscription_data[items][][quantity]": args.Quantity,
	}
	if mainSite.Stripe != nil && !mainSite.FreePlan() {
		body["customer"] = *mainSite.Stripe
	} else {
		body["customer_email"] = goatcounter.GetUser(r.Context()).Email
	}

	var id zstripe.ID
	_, err = zstripe.Request(&id, "POST", "/v1/checkout/sessions", body.Encode())
	if err != nil {
		return errors.Errorf("zstripe failed: %w; body: %s", err, body.Encode())
	}

	mainSite.PlanPending = &args.Plan
	err = mainSite.UpdateStripe(r.Context())
	if err != nil {
		return err
	}

	return zhttp.JSON(w, id)
}

func (h billing) manage(w http.ResponseWriter, r *http.Request) error {
	mainSite, err := MainSite(r.Context())
	if err != nil {
		return err
	}

	if mainSite.Stripe == nil {
		return guru.New(400, "no Stripe customer for this account?")
	}

	var s struct {
		URL string `json:"url"`
	}
	_, err = zstripe.Request(&s, "POST", "/v1/billing_portal/sessions", zstripe.Body{
		"customer":   *mainSite.Stripe,
		"return_url": mainSite.URL(r.Context()) + "/billing",
	}.Encode())
	if err != nil {
		return err
	}
	return zhttp.SeeOther(w, s.URL)
}

type Session struct {
	ClientReferenceID string `json:"client_reference_id"`
	Customer          string `json:"customer"`
	AmountTotal       int    `json:"amount_total"`
	Currency          string `json:"currency"`
}

func (h billing) webhook(w http.ResponseWriter, r *http.Request) error {
	var event zstripe.Event
	err := event.Read(r)
	if err != nil {
		return err
	}

	var f func(zstripe.Event, http.ResponseWriter, *http.Request) error
	switch event.Type {
	default:
		return zhttp.String(w, "not handling this webhook")

	case "checkout.session.completed":
		f = h.webhookCheckout
	case "customer.subscription.updated":
		f = h.webhookUpdate
	case "customer.subscription.deleted":
		f = h.webhookDelete
	}

	err = f(event, w, r)
	if err != nil {
		zlog.Module("billing").FieldsRequest(r).Field("json", string(event.Data.Raw)).Error(err)
		return guru.WithCode(400, err)
	}
	return zhttp.String(w, "okay")
}

type Subscription struct {
	CancelAt zjson.Timestamp `json:"cancel_at"`
	Customer string          `json:"customer"`
	Items    struct {
		Data []struct {
			Quantity int `json:"quantity"`
			Price    struct {
				ID         string `json:"id"`
				Currency   string `json:"currency"`
				UnitAmount int    `json:"unit_amount"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

func (h billing) webhookUpdate(event zstripe.Event, w http.ResponseWriter, r *http.Request) error {
	var s Subscription
	err := json.Unmarshal(event.Data.Raw, &s)
	if err != nil {
		return err
	}

	var (
		currency = strings.ToUpper(s.Items.Data[0].Price.Currency)
		amount   = (s.Items.Data[0].Price.UnitAmount * s.Items.Data[0].Quantity) / 100
	)

	plan, err := getPlan(r.Context(), s)
	if err != nil {
		return err
	}

	var site goatcounter.Site
	err = site.ByStripe(r.Context(), s.Customer)
	if err != nil {
		return err
	}

	site.PlanCancelAt = nil
	if !s.CancelAt.IsZero() {
		site.PlanCancelAt = &s.CancelAt.Time
	}

	site.Plan = plan
	site.BillingAmount = zstring.NewPtr(fmt.Sprintf("%s %d", currency, amount)).P
	return site.UpdateStripe(r.Context())
}

func (h billing) webhookDelete(event zstripe.Event, w http.ResponseWriter, r *http.Request) error {
	// I don't think we ever need to handle this, since cancellations are
	// already pushed with an update. But log for now just in case.
	fmt.Println(string(event.Data.Raw))
	return nil
}

func getPlan(ctx context.Context, s Subscription) (string, error) {
	planID := s.Items.Data[0].Price.ID
	for k, v := range stripePlans[goatcounter.Config(ctx).Dev] {
		if v == planID {
			return k, nil
		}
	}
	return "", fmt.Errorf("unknown plan: %q", planID)
}

func (h billing) webhookCheckout(event zstripe.Event, w http.ResponseWriter, r *http.Request) error {
	var s Session
	err := json.Unmarshal(event.Data.Raw, &s)
	if err != nil {
		return err
	}

	// No processing needed for one-time donations.
	if strings.HasPrefix(s.ClientReferenceID, "one-time") {
		bgrun.Run("email:donation", func() {
			t := "New one-time donation: " + s.ClientReferenceID
			blackmail.Send(t,
				blackmail.From("GoatCounter Billing", "billing@goatcounter.com"),
				blackmail.To("billing@goatcounter.com"),
				blackmail.Bodyf(t))
		})
		return nil
	}

	id, err := strconv.ParseInt(s.ClientReferenceID, 10, 64)
	if err != nil {
		return fmt.Errorf("ClientReferenceID: %w", err)
	}

	var site goatcounter.Site
	err = site.ByID(r.Context(), id)
	if err != nil {
		return err
	}

	site.Stripe = &s.Customer
	site.BillingAmount = zstring.NewPtr(fmt.Sprintf("%s %d", strings.ToUpper(s.Currency), s.AmountTotal/100)).P
	site.Plan = *site.PlanPending
	site.PlanPending = nil
	site.PlanCancelAt = nil
	return site.UpdateStripe(r.Context())
}
