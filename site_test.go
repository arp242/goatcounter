// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/zvalidate"
)

func TestSiteInsert(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	s := Site{Code: "the-code", Plan: PlanPersonal}
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
			Site{Code: "hello-0", State: StateActive, Plan: PlanPersonal},
			nil,
			nil,
		},
		{
			Site{Code: "h€llo", State: StateActive, Plan: PlanPersonal},
			nil,
			map[string][]string{"code": {"must be a valid hostname: invalid character: '€'"}},
		},
		{
			Site{Code: "hel_lo", State: StateActive, Plan: PlanPersonal},
			nil,
			map[string][]string{"code": {"must be a valid hostname: invalid character: '_'"}},
		},
		{
			Site{Code: "hello", State: StateActive, Plan: PlanPersonal},
			func(ctx context.Context) {
				s := Site{Code: "hello", State: StateActive, Plan: PlanPersonal}
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
			ctx, clean := gctest.DB(t)
			defer clean()

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
