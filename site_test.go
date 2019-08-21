package goatcounter

import (
	"testing"
)

func TestSiteInsert(t *testing.T) {
	ctx, clean := StartTest(t)
	defer clean()

	s := Site{
		Code:    "the_code",
		Plan:    PlanPersonal,
		Domains: Domains{Domain{Domain: "hello.com"}},
	}
	err := s.Insert(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if s.ID == 0 {
		t.Fatal("ID is 0")
	}
	// Make sure we insert all domains.
	if len(s.Domains) != 1 {
		t.Fatal()
	}
	if s.Domains[0].Domain != "hello.com" {
		t.Fatal()
	}

	t.Run("update", func(t *testing.T) {
		// Add domain, update existing domain.
		s.Domains[0].Domain = "updated.com"
		s.Domains = append(s.Domains, Domain{Domain: "new.com"})
		err := s.Update(ctx)
		if err != nil {
			t.Fatal(err)
		}

		var ns Site
		err = ns.ByID(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}

		if len(ns.Domains) != 2 {
			t.Fatal()
		}
		if ns.Domains[0].Domain != "updated.com" {
			t.Errorf("domain not updated: %#v\n", ns.Domains[0])
		}
		if ns.Domains[1].Domain != "new.com" {
			t.Errorf("domain not added: %#v\n", ns.Domains[1])
		}

		// Remove a domain.
		ns.Domains = ns.Domains[1:]
		err = ns.Update(ctx)
		if err != nil {
			t.Fatal(err)
		}

		var ns2 Site
		err = ns2.ByID(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}

		if len(ns2.Domains) != 1 {
			t.Fatal()
		}
		if ns2.Domains[0].Domain != "new.com" {
			t.Errorf("wrong domain: %#v\n", ns.Domains[0])
		}
	})
}

func TestSiteByID(t *testing.T) {
	ctx, clean := StartTest(t)
	defer clean()

	var s Site
	err := s.ByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}

	// Just make sure we fetch all domains.
	if len(s.Domains) != 2 {
		t.Fatalf("wrong domains: %#v\n", s.Domains)
	}
}

func TestSitesList(t *testing.T) {
	ctx, clean := StartTest(t)
	defer clean()

	_, err := MustGetDB(ctx).ExecContext(ctx, `insert into sites (code, plan, settings, created_at) values
		('another', 'p', '{}', datetime());`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = MustGetDB(ctx).ExecContext(ctx, `insert into domains (site, domain, created_at) values
		(2, 'w00t.com', datetime()), (2, 'wut.net', datetime());`)
	if err != nil {
		t.Fatal(err)
	}

	var s Sites
	err = s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(s) != 2 {
		t.Fatal()
	}

	// Just make sure we fetch all domains.
	if len(s[0].Domains) != 2 {
		t.Fatal()
	}
	if len(s[1].Domains) != 2 {
		t.Fatal()
	}
	if s[0].Domains[1].Domain != "example.net" {
		t.Fatal()
	}
	if s[1].Domains[1].Domain != "wut.net" {
		t.Fatal()
	}
}
