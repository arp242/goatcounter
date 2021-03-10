// Copyright © 2019 Martin Tournoij <martin@arp242.net>
// This file is part of GoatCounter and published under the terms of the EUPL
// v1.2, which can be found in the LICENSE file or at http://eupl12.zgo.at

package logscan

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"zgo.at/zstd/ztest"
)

func TestErrors(t *testing.T) {
	_, err := New(strings.NewReader(""), "log:$xxx", "", "", "")
	if !ztest.ErrorContains(err, "unknown format specifier: $xxx") {
		t.Error(err)
	}

	_, err = New(strings.NewReader(""), "xxx", "", "", "")
	if !ztest.ErrorContains(err, "unknown format: xxx") {
		t.Error(err)
	}
}

func TestNew(t *testing.T) {
	files, err := ioutil.ReadDir("./testdata")
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{
		{
			"datetime":    "10/Oct/2000:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/test.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"referrer":    "http://www.example.com/start.html",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2000:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/test.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"referrer":    "",
			"user_agent":  "",
		},
	}

	for _, f := range files {
		t.Run(f.Name(), func(t *testing.T) {
			fp, err := os.Open("./testdata/" + f.Name())
			if err != nil {
				t.Fatal(err)
			}

			scan, err := New(fp, f.Name(), "", "", "")
			if err != nil {
				t.Fatal(err)
			}

			i := 0
			for {
				data, err := scan.Line(context.Background())
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}

				w := make(Line)
				for k, v := range want[i] {
					w[k] = v
				}
				switch f.Name() { // This is not in the file
				case "combined":
					delete(w, "host")
				case "common":
					delete(w, "host")
					delete(w, "referrer")
					delete(w, "user_agent")
				case "common-vhost":
					delete(w, "referrer")
					delete(w, "user_agent")
				}

				if !reflect.DeepEqual(data, w) {
					t.Errorf("\nwant: %v\ngot:  %v", w, data)
				}

				dt, err := data.Datetime(scan)
				if err != nil {
					t.Logf("%q %q %q", w["date"], w["time"], w["datetime"])
					t.Fatal(err)
				}

				fmt.Println(dt)
				i++
			}
		})
	}
}

func TestNewFollow(t *testing.T) {
	lines := []string{
		`example.com:127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /test.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
		`example.com:127.0.0.1 - - [10/Oct/2001:13:55:36 -0700] "GET /test.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
		`example.com:127.0.0.1 - - [10/Oct/2001:13:55:36 -0700] "GET /other.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
		`example.org:127.0.0.1 - - [10/Oct/2001:13:55:36 -0700] "GET /other.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
	}

	tmp := ztest.TempFile(t, lines[0]+"\n")

	ctx, stop := context.WithCancel(context.Background())

	scan, err := NewFollow(ctx, tmp, "combined-vhost", "", "", "")
	if err != nil {
		t.Fatal(err)
	}

	echo := func(line string) {
		cmd := exec.Command("tee", "-a", tmp)
		cmd.Stdin = strings.NewReader(line + "\n")
		err := cmd.Run()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Swap out file.
	os.Remove(tmp)
	fp, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fp.Close()

	go func() {
		// inotify does weird stuff with file descriptors; so write to it from a
		// diferent process.
		time.Sleep(200 * time.Millisecond)
		for _, l := range lines {
			time.Sleep(10 * time.Millisecond)
			echo(l)
		}

		time.Sleep(10 * time.Millisecond)
		for _, l := range lines {
			time.Sleep(10 * time.Millisecond)
			echo(l)
		}

		stop()
	}()

	data := []Line{}
	for {
		line, err := scan.Line(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		data = append(data, line)
	}

	if len(data) != 8 {
		t.Fatal(len(data))
	}

	want := []Line{
		{
			"datetime":    "10/Oct/2000:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/test.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2001:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/test.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2001:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/other.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2001:13:55:36 -0700",
			"host":        "example.org",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/other.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2000:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/test.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2001:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/test.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2001:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/other.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
		{
			"datetime":    "10/Oct/2001:13:55:36 -0700",
			"host":        "example.org",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/other.html",
			"referrer":    "http://www.example.com/start.html",
			"remote_addr": "127.0.0.1",
			"size":        "2326",
			"status":      "200",
			"user_agent":  "Mozilla/5.0",
		},
	}

	for i := range data {
		if !reflect.DeepEqual(data[i], want[i]) {
			t.Errorf("line %d\nwant: %#v\ngot:  %#v", i, want[i], data[i])
		}
	}
}
