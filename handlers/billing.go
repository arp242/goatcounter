// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

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
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/bgrun"
	"zgo.at/guru"
	"zgo.at/json"
	"zgo.at/zhttp"
	"zgo.at/zhttp/mware"
	"zgo.at/zlog"
	"zgo.at/zstd/zbool"
	"zgo.at/zstd/zjson"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztime"
	"zgo.at/zstd/ztype"
	"zgo.at/zstripe"
)

var stripePlans = map[bool]map[string]string{
	false: { // Production
		goatcounter.PlanPersonal:     "plan_GLVKIvCvCjzT2u",
		goatcounter.PlanStarter:      "plan_GlJxixkxNZZOct",
		goatcounter.PlanBusiness:     "plan_GLVGCVzLaPA3cY",
		goatcounter.PlanBusinessPlus: "plan_GLVHJUi21iV4Wh",
		"donate":                     "sku_H9jE6zFGzh6KKb",
		"pageviews":                  "price_1IcOIeE5ASM7XVaUA9fNx6dp",
	},
	true: { // Test data
		goatcounter.PlanPersonal:     "price_1IVVd4E5ASM7XVaUGld6Fwys",
		goatcounter.PlanStarter:      "price_1IVVbtE5ASM7XVaUABPNZewg",
		goatcounter.PlanBusiness:     "price_1IVVh1E5ASM7XVaUK3q5leCi",
		goatcounter.PlanBusinessPlus: "price_1IVVg6E5ASM7XVaUZrJwlrfn",
		"donate":                     "sku_J7l6316cimcIC5",
		"pageviews":                  "price_1IeaEwE5ASM7XVaUK4S9ETU2",
	},
}

type billing struct{}

func (h billing) mount(pub, auth chi.Router) {
	auth = auth.With(requireAccess(goatcounter.AccessAdmin))

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
	auth.Post("/billing/extra", zhttp.Wrap(h.extra))
}

func (h billing) index(w http.ResponseWriter, r *http.Request) error {
	account := Account(r.Context())

	switch r.URL.Query().Get("return") {
	case "cancel":
		zhttp.FlashError(w, T(r.Context(), "error/payment-cancelled|Payment cancelled."))

	case "success":
		// Verify that the webhook was processed correct.
		if !account.Subscribed() {
			zhttp.Flash(w, T(r.Context(), "notify/payment-processing|The payment processor reported success, but we're still processing the payment"))
			zlog.Fields(zlog.F{
				"siteID":   account.ID,
				"stripeID": ztype.Ptr(account.Stripe),
			}).Errorf("stripe not processed")
		} else {
			bgrun.Run("email:subscription", func() {
				blackmail.Send("New GoatCounter subscription "+account.Plan,
					blackmail.From("GoatCounter Billing", "billing@goatcounter.com"),
					blackmail.To("billing@goatcounter.com"),
					blackmail.Bodyf(`New subscription: %s (%d) %s`, account.Code, account.ID, *account.Stripe))
			})
			zhttp.Flash(w, T(r.Context(), "notify/payment-processed|Payment processed successfully!"))
		}
	}

	var usage goatcounter.AccountUsage
	err := usage.Get(r.Context())
	if err != nil {
		return err
	}

	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
		Account         *goatcounter.Site
		Usage           goatcounter.AccountUsage
		StripePublicKey string
	}{newGlobals(w, r), account, usage, zstripe.PublicKey})
}

func (h billing) start(w http.ResponseWriter, r *http.Request) error {
	account := Account(r.Context())

	var args struct {
		Plan     string     `json:"plan"`
		Quantity string     `json:"quantity"`
		NoDonate zbool.Bool `json:"nodonate"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	v := goatcounter.NewValidate(r.Context())
	v.Required("plan", args.Plan)
	v.Include("plan", args.Plan, goatcounter.PlanCodes)
	v.Required("quantity", args.Quantity)
	v.Integer("quantity", args.Quantity)
	if v.HasErrors() {
		return v
	}

	// Don't need to send anything to Stripe.
	if args.NoDonate || (args.Plan == goatcounter.PlanPersonal && args.Quantity == "0") {
		account.Plan = goatcounter.PlanFree
		account.Stripe = nil
		account.BillingAmount = nil
		account.PlanPending = nil
		account.PlanCancelAt = nil
		err := account.UpdateStripe(r.Context())
		if err != nil {
			return err
		}
		zhttp.Flash(w, T(r.Context(), "notify/saved|Saved!"))
		return zhttp.JSON(w, `{"status":"ok","no_stripe":true}`)
	}

	body := zstripe.Body{
		"mode":                                 "subscription",
		"payment_method_types[]":               "card",
		"client_reference_id":                  strconv.FormatInt(account.ID, 10),
		"success_url":                          account.URL(r.Context()) + "/billing?return=success",
		"cancel_url":                           account.URL(r.Context()) + "/billing?return=cancel",
		"subscription_data[items][][plan]":     stripePlans[goatcounter.Config(r.Context()).Dev][args.Plan],
		"subscription_data[items][][quantity]": args.Quantity,
		"subscription_data[metadata][site_id]": strconv.FormatInt(account.ID, 10),
	}
	if account.StripeCustomer() {
		body["customer"] = *account.Stripe
	} else {
		body["customer_email"] = User(r.Context()).Email
	}

	var id zstripe.ID
	_, err = zstripe.Request(&id, "POST", "/v1/checkout/sessions", body.Encode())
	if err != nil {
		return errors.Errorf("zstripe failed: %w; body: %s", err, body.Encode())
	}

	account.PlanPending = &args.Plan
	err = account.UpdateStripe(r.Context())
	if err != nil {
		return err
	}

	return zhttp.JSON(w, id)
}

func (h billing) extra(w http.ResponseWriter, r *http.Request) error {
	var args struct {
		AllowExtra bool `json:"allow_extra"`
		MaxExtra   int  `json:"max_extra"`
	}
	_, err := zhttp.Decode(r, &args)
	if err != nil {
		return err
	}

	account := Account(r.Context())
	pvPrice := stripePlans[goatcounter.Config(r.Context()).Dev]["pageviews"]

	var sub struct {
		Data []struct {
			ID    string `json:"id"`
			Items struct {
				Data []struct {
					ID   string `json:"id"`
					Plan struct {
						ID string `json:"id"`
					} `json:"plan"`
				} `json:"data"`
			} `json:"items"`
		} `json:"data"`
	}
	_, err = zstripe.Request(&sub, "GET", "/v1/subscriptions", zstripe.Body{
		"customer": *account.Stripe,
	}.Encode())
	if err != nil {
		return err
	}
	if len(sub.Data) == 0 {
		return guru.New(400, "no subscriptions found")
	}

	found := ""
	for _, item := range sub.Data[0].Items.Data {
		if item.Plan.ID == pvPrice {
			found = item.ID
			break
		}
	}

	var si zstripe.ID
	if args.AllowExtra && found == "" {
		_, err = zstripe.Request(&si, "POST", "/v1/subscription_items", zstripe.Body{
			"subscription": sub.Data[0].ID,
			"price":        pvPrice,
		}.Encode())
	} else if !args.AllowExtra && found != "" {
		_, err = zstripe.Request(&si, "DELETE", "/v1/subscription_items/"+found, zstripe.Body{
			"clear_usage":    "true",
			"proration_date": strconv.FormatInt(account.NextInvoice().Unix(), 10),
		}.Encode())
	}
	if err != nil {
		return err
	}

	account.ExtraPageviews = nil
	account.ExtraPageviewsSub = nil
	if args.AllowExtra {
		account.ExtraPageviews = &args.MaxExtra
		account.ExtraPageviewsSub = &found
		if found == "" {
			account.ExtraPageviewsSub = &si.ID
		}
	}
	err = account.UpdateStripe(r.Context())
	if err != nil {
		return err
	}

	zhttp.Flash(w, "Saved!")
	return zhttp.SeeOther(w, "/billing")
}

func (h billing) manage(w http.ResponseWriter, r *http.Request) error {
	account := Account(r.Context())

	if account.Stripe == nil {
		return guru.New(400, "no Stripe customer for this account?")
	}

	var s struct {
		URL string `json:"url"`
	}
	_, err := zstripe.Request(&s, "POST", "/v1/billing_portal/sessions", zstripe.Body{
		"customer":   *account.Stripe,
		"return_url": account.URL(r.Context()) + "/billing",
	}.Encode())
	if err != nil {
		return err
	}
	return zhttp.SeeOther(w, s.URL)
}

type (
	// Session https://stripe.com/docs/api/checkout/sessions/object
	Session struct {
		ClientReferenceID string `json:"client_reference_id"`
		Customer          string `json:"customer"`
		AmountTotal       int    `json:"amount_total"`
		Currency          string `json:"currency"`
	}

	// Subscription https://stripe.com/docs/api/subscriptions/object
	Subscription struct {
		CancelAt           zjson.Timestamp `json:"cancel_at"`
		BillingCycleAnchor zjson.Timestamp `json:"billing_cycle_anchor"`
		Customer           string          `json:"customer"`
		Metadata           struct {
			SiteID zjson.Int `json:"site_id"`
		} `json:"metadata"`
		Items struct {
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
)

func (h billing) webhook(w http.ResponseWriter, r *http.Request) error {
	var event zstripe.Event
	err := event.Read(r)
	if err != nil {
		return err
	}

	f, ok := map[string]func(zstripe.Event, http.ResponseWriter, *http.Request) error{
		zstripe.EventCheckoutSessionCompleted:    h.whCheckout,
		zstripe.EventCustomerSubscriptionCreated: h.whSubscriptionUpdated,
		zstripe.EventCustomerSubscriptionUpdated: h.whSubscriptionUpdated,
		zstripe.EventCustomerSubscriptionDeleted: h.whSubscriptionDeleted,
	}[event.Type]
	if !ok {
		w.WriteHeader(202)
		return zhttp.String(w, "not handling this webhook")
	}

	err = f(event, w, r)
	if err != nil {
		zlog.Module("billing").FieldsRequest(r).Field("json", string(event.Data.Raw)).Error(err)
		return guru.WithCode(400, err)
	}
	return zhttp.String(w, "okay")
}

func (h billing) whSubscriptionUpdated(event zstripe.Event, w http.ResponseWriter, r *http.Request) error {
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

	// In theory we should be able to get the Stripe customer ID from the
	// checkout.session.completed webhook, but for some reason this isn't always
	// sent. On the created event we also have the metadata.site_id.
	if err != nil && s.Metadata.SiteID > 0 {
		err = site.ByID(r.Context(), int64(s.Metadata.SiteID))
	}
	if err != nil {
		return fmt.Errorf("whSubscriptionUpdated: cannot find Stripe customer %q (metadata: %d), %w",
			s.Customer, s.Metadata.SiteID, err)
	}

	site.PlanCancelAt = nil
	if !s.CancelAt.IsZero() {
		site.PlanCancelAt = &s.CancelAt.Time
	}

	if site.Stripe == nil {
		site.Stripe = &s.Customer
	}
	site.Plan = plan
	site.BillingAmount = ztype.Ptr(fmt.Sprintf("%s %d", currency, amount))
	if s.BillingCycleAnchor.IsZero() {
		site.BillingAnchor = nil
	} else {
		site.BillingAnchor = &s.BillingCycleAnchor.Time
	}
	return site.UpdateStripe(r.Context())
}

func (h billing) whSubscriptionDeleted(event zstripe.Event, w http.ResponseWriter, r *http.Request) error {
	// I don't think we ever need to handle this, since cancellations are
	// already pushed with an update. But log for now just in case.
	fmt.Println(string(event.Data.Raw))
	return nil
}

func (h billing) whCheckout(event zstripe.Event, w http.ResponseWriter, r *http.Request) error {
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

	n := ztime.Now()
	site.Stripe = &s.Customer
	site.BillingAmount = ztype.Ptr(fmt.Sprintf("%s %d", strings.ToUpper(s.Currency), s.AmountTotal/100))
	site.BillingAnchor = &n
	site.Plan = *site.PlanPending
	site.PlanPending = nil
	site.PlanCancelAt = nil
	return site.UpdateStripe(r.Context())
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
