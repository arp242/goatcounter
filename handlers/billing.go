// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the AGPLv3,
// which can be found in the LICENSE file or at gnu.org/licenses/agpl.html

package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi"
	"zgo.at/goatcounter"
	"zgo.at/zhttp"
	"zgo.at/zstripe"
)

type billing struct{}

func (h billing) mount(r chi.Router) {
	r.Get("/billing", zhttp.Wrap(h.index))         // Overview of status, invoices
	r.Get("/billing/start", zhttp.Wrap(h.start))   // create Stripe plan
	r.Get("/billing/stop", zhttp.Wrap(h.stop))     // Cancel
	r.Get("/billing/change", zhttp.Wrap(h.change)) // Change plan, CC, etc.
}

func (h billing) index(w http.ResponseWriter, r *http.Request) error {
	return zhttp.Template(w, "billing.gohtml", struct {
		Globals
	}{newGlobals(w, r)})
}

func (h billing) start(w http.ResponseWriter, r *http.Request) error {
	type Card struct {
		Number   string `json:"number"`    // "4242424242"
		ExpMonth string `json:"exp_month"` // "11"
		ExpYear  string `json:"exp_year"`  // "2020"
		CVC      string `json:"cvc"`       // "314"
	}
	var card Card
	_, err := zhttp.Decode(r, &card)
	if err != nil {
		return err
	}

	site := goatcounter.MustGetSite(r.Context())

	// Create customer and payment method if there isn't one yet.
	if site.Stripe == nil {
		type Customer struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		c := Customer{}
		body := make(url.Values)
		body.Set("name", "Martin Tournoij")
		body.Set("email", "martin@arp242.net")
		_, err := zstripe.Request(&c, "POST", "/v1/customers", strings.NewReader(body.Encode()))
		if err != nil {
			return err
		}

		site.Stripe = &c.ID
		err = site.UpdateStripe(r.Context())
		if err != nil {
			return err
		}

		type PaymentMethod struct {
			ID       string `json:"id"`
			Type     string `json:"type"`     // "card"
			Customer string `json:"customer"` // "cus_..."
			Card     Card   `json:"card"`
		}
		p := PaymentMethod{
			Type:     "card",
			Customer: c.ID,
			Card:     card,
		}

		body = make(url.Values)
		body.Set("type", p.Type)
		body.Set("customer", p.Customer)
		body.Set("card.number", p.Card.Number)
		body.Set("card.exp_month", p.Card.ExpMonth)
		body.Set("card.exp_year", p.Card.ExpYear)
		body.Set("card.cvc", p.Card.CVC)
		_, err = zstripe.Request(&p, "POST", "/v1/payment_methods", strings.NewReader(body.Encode()))
		if err != nil {
			return err
		}

		// customer.invoice_settings.default_payment_method
		// ID of the default payment method used for subscriptions and invoices
		// for the customer.
	}

	// Subscribe customer to the plan.
	type Subscription struct {
		Customer string

		// trial_end timestamp
		//
		// If the subscription has a trial, the end of that trial.
		// trial_start timestamp
		//
		// If the subscription has a trial, the beginning of that trial.

		// Items []struct {
		// 	Data struct {
		// 		Plan string
		// 	}
		// }

		// plan hash, plan object
		//
		// Hash describing the plan the customer is subscribed to. Only set if the
		// subscription contains a single plan.

		//  default_payment_method string
		//
		// ID of the default payment method for the subscription. It must belong
		// to the customer associated with the subscription. If not set,
		// invoices will use the default payment method in the customer’s
		// invoice settings.

		// billing_cycle_anchor timestamp
		// Determines the date of the first full invoice, and, for plans with month
		// or year intervals, the day of the month for subsequent invoices.

		//collection_method string
		// Either charge_automatically, or send_invoice. When charging automatically,
		// Stripe will attempt to pay this subscription at the end of the cycle using
		// the default source attached to the customer. When sending an invoice, Stripe
		// will email your customer an invoice with payment instructions.
	}
	s := Subscription{}
	body := make(url.Values)
	_, err = zstripe.Request(&s, "POST", "/v1/subscriptions", strings.NewReader(body.Encode()))
	if err != nil {
		return err
	}

	// items := []*stripe.SubscriptionItemsParams{
	//   {
	//     Plan: stripe.String("plan_FSDjyHWis0QVwl"),
	//   },
	// }
	// params := &stripe.SubscriptionParams{
	//   Customer: stripe.String("cus_G02hIo15n8CU1s"),
	//   Items: items,
	// }
	// params.AddExpand("latest_invoice.payment_intent")
	// subscription, _ := sub.New(params)

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
