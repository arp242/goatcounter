// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"zgo.at/zvalidate"
)

func TestSiteInsert(t *testing.T) {
	ctx, clean := StartTest(t)
	defer clean()

	s := Site{Code: "the_code", Name: "the-code.com", Plan: PlanPersonal}
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
			Site{Name: "Hello", Code: "hello-_0", State: StateActive, Plan: PlanPersonal},
			nil,
			nil,
		},
		{
			Site{Name: "Hello", Code: "h€llo", State: StateActive, Plan: PlanPersonal},
			nil,
			map[string][]string{"code": {"'€' not allowed; characters are limited to '_', '-', a to z, and numbers"}},
		},
		{
			Site{Name: "Hello", Code: "-hello", State: StateActive, Plan: PlanPersonal},
			nil,
			map[string][]string{"code": {"cannot start with underscore or dash (_, -)"}},
		},
		{
			Site{Name: "Hello", Code: "hello", State: StateActive, Plan: PlanPersonal},
			func(ctx context.Context) {
				s := Site{Name: "Hello", Code: "hello", State: StateActive, Plan: PlanPersonal}
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
			ctx, clean := StartTest(t)
			defer clean()

			if tt.prefn != nil {
				tt.prefn(ctx)
			}

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
