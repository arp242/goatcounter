// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package goatcounter_test

import (
	"reflect"
	"strings"
	"testing"

	. "zgo.at/goatcounter"
	"zgo.at/goatcounter/gctest"
	"zgo.at/isbot"
	"zgo.at/zdb"
	"zgo.at/zstd/ztest"
)

func TestUserAgentGetOrInsert(t *testing.T) {
	ctx, clean := gctest.DB(t)
	defer clean()

	test := func(gotUA, wantUA UserAgent, want string) {
		if !reflect.DeepEqual(gotUA, wantUA) {
			t.Fatalf("wrong ua\ngot:  %#v\nwant: %#v", gotUA, wantUA)
		}

		want = strings.ReplaceAll(strings.TrimSpace(strings.ReplaceAll(want, "\t", "")), "@", " ")
		out := zdb.DumpString(ctx, `select * from user_agents`) +
			zdb.DumpString(ctx, `select * from browsers`) +
			zdb.DumpString(ctx, `select * from systems`)
		if d := ztest.Diff(out, want); d != "" {
			t.Errorf(d)
		}
	}

	{
		ua := UserAgent{UserAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0"}
		err := ua.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 1, BrowserID: 1, SystemID: 1, Bot: isbot.NoBotNoMatch}, `
			user_agent_id  ua                                              bot  browser_id  system_id
			1              ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0  1    1           1
			browser_id  name     version
			1           Firefox  79
			system_id  name   version
			1          Linux
		`)
	}

	{
		ua := UserAgent{UserAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0"}
		err := ua.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 1, BrowserID: 1, SystemID: 1, Bot: isbot.NoBotNoMatch}, `
			user_agent_id  ua                                              bot  browser_id  system_id
			1              ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0  1    1           1
			browser_id  name     version
			1           Firefox  79
			system_id  name   version
			1          Linux
		`)
	}

	{
		ua := UserAgent{UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:79.0) Gecko/20100101 Firefox/79.0"}
		err := ua.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 2, BrowserID: 1, SystemID: 2, Bot: isbot.NoBotNoMatch}, `
			user_agent_id  ua                                                      bot  browser_id  system_id
			1              ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0          1    1           1
			2              ~Z (~W NT 10.0; Win64; x64; rv:79.0) ~g20100101 ~f79.0  1    1           2
			browser_id  name     version
			1           Firefox  79
			system_id  name     version
			1          Linux@@@@
			2          Windows  10
		`)
	}

	{
		ua := UserAgent{UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:71.0) Gecko/20100101 Firefox/71.0"}
		err := ua.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 3, BrowserID: 2, SystemID: 2, Bot: isbot.NoBotNoMatch}, `
			user_agent_id  ua                                                      bot  browser_id  system_id
			1              ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0          1    1           1
			2              ~Z (~W NT 10.0; Win64; x64; rv:79.0) ~g20100101 ~f79.0  1    1           2
			3              ~Z (~W NT 10.0; Win64; x64; rv:71.0) ~g20100101 ~f71.0  1    2           2
			browser_id  name     version
			1           Firefox  79
			2           Firefox  71
			system_id  name     version
			1          Linux@@@@
			2          Windows  10
		`)
	}
}
