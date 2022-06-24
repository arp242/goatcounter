// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/zvalidate"
)

func TestGetAccount(t *testing.T) {
	ctx := gctest.DB(t)

	a := MustGetAccount(ctx)
	if a.ID != MustGetSite(ctx).ID {
		t.Fatal()
	}

	ctx2 := gctest.Site(ctx, t, &Site{Parent: &MustGetSite(ctx).ID}, nil)
	a2 := MustGetAccount(ctx2)
	if a2.ID != MustGetSite(ctx).ID {
		t.Fatal()
	}

	if MustGetSite(ctx).ID != 1 || MustGetSite(ctx2).ID != 2 {
		t.Fatal() // Make sure original isn't modified.
	}
}

func TestSiteInsert(t *testing.T) {
	ctx := gctest.DB(t)

	s := Site{Code: "the-code"}
	err := s.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if s.ID == 0 {
		t.Fatal("ID is 0")
	}
}

func TestSiteValidate(t *testing.T) {
	tests := []struct {
		in    Site
		prefn func(context.Context)
		want  map[string][]string
	}{
		{
			Site{Code: "hello-0", State: StateActive},
			nil,
			nil,
		},
		{
			Site{Code: "h€llo", State: StateActive},
			nil,
			map[string][]string{"code": {"must be a valid hostname: invalid character: '€'"}},
		},
		{
			Site{Code: "hel_lo", State: StateActive},
			nil,
			map[string][]string{"code": {"must be a valid hostname: invalid character: '_'"}},
		},
		{
			Site{Code: "hello", State: StateActive},
			func(ctx context.Context) {
				s := Site{Code: "hello", State: StateActive}
				err := s.Insert(ctx)
				if err != nil {
					panic(err)
				}
			},
			map[string][]string{"code": {"already exists"}},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			ctx := gctest.DB(t)

			if tt.prefn != nil {
				tt.prefn(ctx)
			}

			tt.in.Defaults(ctx)
			err := tt.in.Validate(ctx)
			if err == nil && tt.want == nil {
				return
			}

			verr, ok := err.(*zvalidate.Validator)
			if !ok {
				t.Fatalf("unexpected error type %T: %#[1]v", err)
			}

			if !reflect.DeepEqual(verr.Errors, tt.want) {
				t.Errorf("wrong error\nout:  %s\nwant: %s", verr.Errors, tt.want)
			}
		})
	}
}
