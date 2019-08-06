package stripe

import (
	"context"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
	"github.com/stripe/stripe-go/sub"

	"zgo.at/goatcounter"
)

const (
	StripePlan = "plan_"
)

func init() {
	// TODO
	stripe.Key = "sk_test_1Dexvk0UoDE6fa5St9ob2B5W00huIXcLaT"
}

// Create a new Stripe customer and subscription.
func Create(ctx context.Context, site goatcounter.Site, user goatcounter.User) error {
	params := &stripe.CustomerParams{
		Name:  &user.Name,
		Email: &user.Email,
	}
	params.SetSource("src_18eYalAHEMiOZZp1l9ZTjSU0") // TODO
	customer, err := customer.New(params)
	if err != nil {
		return err
	}

	site.Stripe = &customer.ID
	err = site.UpdateStripe(ctx)
	if err != nil {
		return err
	}

	_, err = sub.New(&stripe.SubscriptionParams{
		Customer: &customer.ID,
		Items: []*stripe.SubscriptionItemsParams{{
			Plan: stripe.String("plan_CBXbz9i7AIOTzr"), // TODO
		}}})
	return err
}
