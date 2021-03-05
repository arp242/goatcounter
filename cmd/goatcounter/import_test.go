// Copyright © 2019 Martin Tournoij – This file is part of GoatCounter and
// published under the terms of a slightly modified EUPL v1.2 license, which can
// be found in the LICENSE file or at https://license.goatcounter.com

package main

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"zgo.at/goatcounter"
	"zgo.at/goatcounter/cron"
	"zgo.at/zdb"
)

func TestImport(t *testing.T) {
	/*
		exit, _, _, ctx, dbc, clean := startTest(t)
		defer clean()
		db := zdb.MustGetDB(ctx)

		// Start import server
		key := goatcounter.APIToken{SiteID: 1, UserID: 1, Name: "test",
			Permissions: goatcounter.APITokenPermissions{Count: true}}
		err := key.Insert(ctx)
		if err != nil {
			t.Fatal(err)
		}

			ready := make(chan struct{}, 1)
			stop := make(chan struct{})
			go runCmdStop(t, exit, ready, stop, "serve",
				"-tls=none",
				"-db="+dbc,
				"-listen=localhost:9876",
				"-debug=all")
			<-ready

			t.Run("csv", func(t *testing.T) {
				goatcounter.Memstore.TestInit(db)
				clean := runImport(ctx, t, key, "./testdata/export.csv")
				defer clean()

				out := zdb.DumpString(ctx, `select * from hits`)
				want := `
						hit_id  site_id  path_id  user_agent_id  session                           bot  ref             ref_scheme  size         location  first_visit  created_at
						1       1        1        1              00112233445566778899aabbccddef01  0                    NULL        1280,768,1   AR        1            2020-12-01 00:07:10
						2       1        2        1              00112233445566778899aabbccddef01  0                    NULL        1280,768,1   AR        1            2020-12-01 00:07:44
						3       1        3        2              00112233445566778899aabbccddef02  0    www.reddit.com  o           1680,1050,2  RO        1            2020-12-27 00:37:37`
				if d := ztest.Diff(out, want, ztest.DiffNormalizeWhitespace); d != "" {
					t.Error(d)
				}
			})

			t.Run("log", func(t *testing.T) {
				goatcounter.Memstore.TestInit(db)
				clean := runImport(ctx, t, key, "-format=combined", "./testdata/access_log")
				defer clean()

				out := zdb.DumpString(ctx, `select * from hits`)
				want := `
				        hit_id  site_id  path_id  user_agent_id  session                           bot  ref                         ref_scheme  size  location  first_visit  created_at
				        1       1        1        1              00112233445566778899aabbccddef01  0    www.example.com/start.html  h                           1            2000-10-10 20:55:36
				        2       1        1        1              00112233445566778899aabbccddef01  0                                NULL                        0            2000-10-10 20:55:36`
				if d := ztest.Diff(out, want, ztest.DiffNormalizeWhitespace); d != "" {
					t.Error(d)
				}
			})

			t.Run("log-follow-100", func(t *testing.T) {
				goatcounter.Memstore.TestInit(db)
				tmp := filepath.Join(t.TempDir(), "access_log")
				fp, err := os.Create(tmp)
				if err != nil {
					t.Fatal(err)
				}
				defer fp.Close()

				var clean func()
				go func() { clean = runImport(ctx, t, key, "-format=combined", "-follow", tmp) }()
				defer clean()
				time.Sleep(1 * time.Second)

				var (
					writeErr error
					wg       sync.WaitGroup
				)
				wg.Add(1)
				go func() {
					defer wg.Done()

					lines := zstring.Repeat(
						`127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /test.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
						100)
					for _, line := range lines {
						_, writeErr = fp.WriteString(line + "\n")
						if writeErr != nil {
							break
						}
					}
					fp.Close()
				}()
				wg.Wait()

				out := zdb.DumpString(ctx, `select * from hits`)
				want := "hit_id  site_id  path_id  user_agent_id  session                           bot  ref                         ref_scheme  size  location  first_visit  created_at\n"
				for i := 1; i < 101; i++ {
					want += fmt.Sprintf(
						"%-3d     1        1        1              00112233445566778899aabbccddef01  0    www.example.com/start.html  h                           0            2000-10-10 20:55:36\n",
						i)

					if i == 1 { // first_visit
						want = strings.Replace(want, "0            ", "1            ", 1)
					}
				}
				if d := ztest.Diff(out, want, ztest.DiffNormalizeWhitespace); d != "" {
					t.Error(d)
				}

				if writeErr != nil {
					t.Fatalf("write error: %s", writeErr)
				}
			})

			t.Run("log-follow-4", func(t *testing.T) {
				tmp := filepath.Join(t.TempDir(), "access_log2")
				fp, err := os.Create(tmp)
				if err != nil {
					t.Fatal(err)
				}
				defer fp.Close()

				var (
					writeErr error
					wg       sync.WaitGroup
				)
				wg.Add(1)
				go func() {
					defer wg.Done()

					lines := zstring.Repeat(
						`127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /test.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
						4)
					for _, line := range lines {
						_, writeErr = fp.WriteString(line + "\n")
						if writeErr != nil {
							break
						}
					}
				}()

				clean := runImport(ctx, t, key, "-format=combined", "-follow", tmp)
				defer clean()

				out := zdb.DumpString(ctx, `select * from hits`)
				want := "hit_id  site_id  path_id  user_agent_id  session                           bot  ref                         ref_scheme  size  location  first_visit  created_at\n"
				for i := 1; i < 5; i++ {
					want += fmt.Sprintf(
						"%-3d     1        1        1              00112233445566778899aabbccddef01  0    www.example.com/start.html  h                           0            2000-10-10 20:55:36\n",
						i)

					if i == 1 { // first_visit
						want = strings.Replace(want, "0            ", "1            ", 1)
					}
				}
				if d := ztest.Diff(out, want, ztest.DiffNormalizeWhitespace); d != "" {
					t.Error(d)
				}

				wg.Wait()
				if writeErr != nil {
					t.Fatalf("write error: %s", writeErr)
				}
			})

			time.Sleep(2 * time.Second)
			stop <- struct{}{}
			mainDone.Wait()
	*/
}

func runImport(ctx context.Context, t *testing.T, key goatcounter.APIToken, args ...string) func() {
	os.Setenv("GOATCOUNTER_API_KEY", key.Token)
	cmd := exec.Command("go", append([]string{"run", ".", "import",
		"-debug=all",
		"-site=http://test.localhost:9876"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(4 * time.Second)
		cmd.Process.Kill()
	}()
	err = cmd.Wait()
	if err != nil {
		if err.Error() != "signal: killed" {
			t.Fatal(err)
		}
	}

	// out, err := cmd.CombinedOutput()
	// if err != nil {
	// 	t.Fatalf("%s: %s", err, out)
	// }
	// if verbose {
	// 	fmt.Println("output:", string(out))
	// }

	err = cron.PersistAndStat(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return func() {
		var paths []int64
		err := zdb.Select(ctx, &paths, `select path_id from paths`)
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) == 0 {
			return
		}
		err = (&goatcounter.Hits{}).Purge(ctx, paths)
		if err != nil {
			t.Fatal(err)
		}

		if zdb.SQLite(ctx) {
			err = zdb.Exec(ctx, `update sqlite_sequence set seq = 0 where name in ('hits', 'paths', 'user_agents')`)
			if err != nil {
				t.Fatal(err)
			}
			err = zdb.Exec(ctx, `delete from user_agents`)
		} else {
			err = zdb.Exec(ctx, `truncate hits, paths, user_agents`)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

/*

// cmd := exec.Command("go", "run", ".", "serve", "-dev", "-db", dbc,
// 	"-listen", "localhost:9876") // TODO: random listen
//
// TODO: figure out a better way to run this server; this is kind of messy.
// I don't really want to run serve() directly either, because there's some
// global state and I'd rather not mix up the states from the import test
// and serve; specifically, the cache for paths and the like.
//
// Also cron memstore should be immediate.
//
// Actually, might be best to run "Serve" in-process here, and "import" as a
// new one.
//
//cmd := exec.Command("go", "run", ".", "serve", "-dev", "-db", dbc,
//	"-listen", "localhost:9876") // TODO: random listen
// cmd.Stdout = os.Stdout
// cmd.Stderr = os.Stderr
// err = cmd.Start()
// if err != nil {
// 	t.Fatal()
// }
// defer func() {
// 	// Doesn't work as "go run" creates a new process.
// 	fmt.Println(cmd.Process.Kill())
// 	fmt.Println(cmd.Process.Signal(os.Interrupt))
// 	fmt.Println(cmd.Process)
// }()
// time.Sleep(3 * time.Second)

// fmt.Printf("%s\n", *site.Cname)
// err = os.Setenv("GOATCOUNTER_API_KEY", key.Token)
// if err != nil {
// 	t.Fatal()
// }
// run(t, 0, []string{"import", "-db", dbc,
// 	"-site", "http://test.localhost:9876",
// 	"cmd/goatcounter/testdata/export.csv"})

// time.Sleep(10 * time.Second)
*/
