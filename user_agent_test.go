// Copyright © Martin Tournoij – This file is part of GoatCounter and published
// under the terms of a slightly modified EUPL v1.2 license, which can be found
// in the LICENSE file or at https://license.goatcounter.com

package goatcounter_test

import (
	"reflect"
	"strings"
	"testing"

	. "zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/gctest"
	"zgo.at/isbot"
	"zgo.at/zdb"
	"zgo.at/zstd/ztest"
)

func TestUserAgentGetOrInsert(t *testing.T) {
	ctx := gctest.DB(t)

	test := func(gotUA, wantUA UserAgent, want string) {
		if !reflect.DeepEqual(gotUA, wantUA) {
			t.Fatalf("wrong ua\ngot:  %#v\nwant: %#v", gotUA, wantUA)
		}

		want = strings.ReplaceAll(strings.TrimSpace(strings.ReplaceAll(want, "\t", "")), "@", " ")
		out := zdb.DumpString(ctx, `
               select
                       user_agents.user_agent_id as id,
                       user_agents.system_id     as bid,
                       user_agents.browser_id    as sid,
                       user_agents.isbot,
                       browsers.name || ' ' || browsers.version as browser,
                       systems.name  || ' ' || systems.version as system,
                       user_agents.ua
               from user_agents
               join browsers using (browser_id)
               join systems using (system_id)`)
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
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 1, BrowserID: 1, SystemID: 1, Isbot: isbot.NoBotNoMatch}, `
			id  bid  sid  isbot  browser     system  ua
			1   1    1    1      Firefox 79  Linux   ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0
		`)
	}

	{
		ua := UserAgent{UserAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0"}
		err := ua.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 1, BrowserID: 1, SystemID: 1, Isbot: isbot.NoBotNoMatch}, `
			id  bid  sid  isbot  browser     system  ua
			1   1    1    1      Firefox 79  Linux   ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0
		`)
	}

	{
		ua := UserAgent{UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:79.0) Gecko/20100101 Firefox/79.0"}
		err := ua.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 2, BrowserID: 1, SystemID: 2, Isbot: isbot.NoBotNoMatch}, `
			id  bid  sid  isbot  browser     system      ua
			1   1    1    1      Firefox 79  Linux       ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0
			2   2    1    1      Firefox 79  Windows 10  ~Z (~W NT 10.0; Win64; x64; rv:79.0) ~g20100101 ~f79.0
		`)
	}

	{
		ua := UserAgent{UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:71.0) Gecko/20100101 Firefox/71.0"}
		err := ua.GetOrInsert(ctx)
		if err != nil {
			t.Fatal(err)
		}
		test(ua, UserAgent{UserAgent: ua.UserAgent, ID: 3, BrowserID: 2, SystemID: 2, Isbot: isbot.NoBotNoMatch}, `
			id  bid  sid  isbot  browser     system      ua
			1   1    1    1      Firefox 79  Linux       ~Z (X11; ~L x86_64; rv:79.0) ~g20100101 ~f79.0
			2   2    1    1      Firefox 79  Windows 10  ~Z (~W NT 10.0; Win64; x64; rv:79.0) ~g20100101 ~f79.0
			3   2    2    1      Firefox 71  Windows 10  ~Z (~W NT 10.0; Win64; x64; rv:71.0) ~g20100101 ~f71.0
		`)
	}
}

func TestUserAgentUpdate(t *testing.T) {
	ctx := gctest.DB(t)

	ua := UserAgent{UserAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:79.0) Gecko/20100101 Firefox/79.0"}
	err := ua.GetOrInsert(ctx)
	if err != nil {
		t.Fatal(err)
	}

	oldB := ua.BrowserID
	oldS := ua.SystemID

	{
		err = ua.Update(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if ua.BrowserID != oldB || ua.SystemID != oldS {
			t.Errorf("browser %d == %d; system %d == %d", oldB, ua.BrowserID, oldS, ua.SystemID)
		}
	}

	{
		ua.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:80.0) Gecko/20100101 Firefox/80.0"
		err = zdb.Exec(ctx, `update user_agents set ua=? where user_agent_id=?`,
			ua.UserAgent, ua.ID)
		if err != nil {
			t.Fatal(err)
		}
		err = ua.Update(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if ua.BrowserID == oldB || ua.SystemID == oldS {
			t.Errorf("browser %d == %d; system %d == %d", oldB, ua.BrowserID, oldS, ua.SystemID)
		}
	}
}
