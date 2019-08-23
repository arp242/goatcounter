package goatcounter

import "testing"

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
