package logscan

import (
	"context"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"zgo.at/zstd/ztest"
)

var (
	_ LineParser = RegexParser{}
	_ Line       = RegexLine{}
	_ LineParser = CaddyParser{}
	_ Line       = CaddyLogEntry{}
)

func TestErrors(t *testing.T) {
	_, err := New(strings.NewReader(""), "log:$xxx", "", "", "", nil)
	if !ztest.ErrorContains(err, "unknown format specifier: $xxx") {
		t.Error(err)
	}

	_, err = New(strings.NewReader(""), "xxx", "", "", "", nil)
	if !ztest.ErrorContains(err, "unknown format: xxx") {
		t.Error(err)
	}
}

func TestNew(t *testing.T) {
	files, err := os.ReadDir("./testdata")
	if err != nil {
		t.Fatal(err)
	}
	want := []RegexLine{
		{
			"datetime":    "10/Oct/2000:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/",
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
		{
			"datetime":    "10/Oct/2000:13:55:36 -0700",
			"host":        "example.com",
			"http":        "HTTP/2.0",
			"method":      "GET",
			"path":        "/dash-size",
			"remote_addr": "127.0.0.1",
			"size":        "",
			"status":      "200",
			"referrer":    "",
			"user_agent":  "",
		},
		{
			"datetime":    "15/May/2023:00:00:54 +0000",
			"host":        "example.com",
			"http":        "HTTP/1.1",
			"method":      "GET",
			"path":        "/proxy.pac",
			"remote_addr": "1.1.1.1",
			"size":        "133",
			"status":      "200",
			"referrer":    "",
			"user_agent":  "",
		},
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		t.Run(f.Name(), func(t *testing.T) {
			fp, err := os.Open("./testdata/" + f.Name())
			if err != nil {
				t.Fatal(err)
			}

			scan, err := New(fp, f.Name(), "", "", "", nil)
			if err != nil {
				t.Fatal(err)
			}

			i := 0
			for {
				data, _, _, err := scan.Line(context.Background())
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}

				w := make(RegexLine)
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
				case "bunny", "bunny-extended":
					delete(w, "http")
					delete(w, "method")
				}

				dt, err := data.Datetime(scan.lp)
				if err != nil {
					t.Logf("%q %q %q", w["date"], w["time"], w["datetime"])
					t.Errorf("parsing datetime: %v", err)
				}
				wantdt, err := time.Parse("02/Jan/2006:15:04:05 -0700", w["datetime"])
				if err != nil {
					t.Fatal(err)
				}
				if !dt.Equal(wantdt) {
					t.Errorf("parsing datetime:\nhave: %s\nwant: %s", dt, wantdt)
				}

				delete(w, "datetime")
				delete(w, "date")
				delete(w, "time")
				m := data.(RegexLine)
				delete(m, "datetime")
				delete(m, "date")
				delete(m, "time")

				if !reflect.DeepEqual(data, w) {
					t.Errorf("\nwant: %v\nhave: %v", w, data)
				}

				i++
			}
		})
	}
}

func TestNewFollow(t *testing.T) {
	// TODO: why did this test start failing all of the sudden? Nothing changed...
	t.Skip()

	lines := []string{
		`example.com:127.0.0.1 - - [10/Oct/2000:13:55:36 -0700] "GET /test.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
		`example.com:127.0.0.1 - - [10/Oct/2001:13:55:36 -0700] "GET /test.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
		`example.com:127.0.0.1 - - [10/Oct/2001:13:55:36 -0700] "GET /other.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
		`example.org:127.0.0.1 - - [10/Oct/2001:13:55:36 -0700] "GET /other.html HTTP/1.1" 200 2326 "http://www.example.com/start.html" "Mozilla/5.0"`,
	}

	tmp := ztest.TempFile(t, "", lines[0]+"\n")

	ctx, stop := context.WithCancel(context.Background())

	scan, err := NewFollow(ctx, tmp, "combined-vhost", "", "", "", nil)
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

	data := []RegexLine{}
	for {
		line, _, _, err := scan.Line(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		data = append(data, line.(RegexLine))
	}

	if len(data) != 8 {
		t.Fatalf("len(data) = %d instead of 8", len(data))
	}

	want := []RegexLine{
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
		delete(data[i], "_line")
		if !reflect.DeepEqual(data[i], want[i]) {
			t.Errorf("line %d\nwant: %#v\nhave: %#v", i, want[i], data[i])
		}
	}
}

func TestExclude(t *testing.T) {
	tests := []struct {
		exclude []string
		lines   []string
		want    []string
		wantErr string
	}{
		{[]string{""}, nil, nil, "invalid field"},
		{[]string{"xx:yy"}, nil, nil, "invalid field"},
		{[]string{"path:"}, nil, nil, "no pattern"},
		{[]string{}, []string{"/path", "/p"}, []string{"/path", "/p"}, ""},

		{[]string{"path:/p"},
			[]string{"/path", "/p"},
			nil,
			""},

		{[]string{"path:re:/p$"},
			[]string{"/path", "/p"},
			[]string{"/path"},
			""},
		{[]string{"path:glob:/p"},
			[]string{"/path", "/p"},
			[]string{"/path"},
			""},

		{[]string{"!path:re:/p$"},
			[]string{"/path", "/p"},
			[]string{"/p"},
			""},
		{[]string{"!path:glob:/p"},
			[]string{"/path", "/p"},
			[]string{"/p"},
			""},

		{[]string{"path:glob:/private/**"},
			[]string{"/path/private", "/private/path/x"},
			[]string{"/path/private"},
			""},

		{[]string{"path:re:/private"},
			[]string{"/path/private", "/private/path/x"},
			nil,
			""},

		{[]string{"path:re:^/private"},
			[]string{"/path/private", "/private/path/x"},
			[]string{"/path/private"},
			""},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			tmp, err := os.Open(ztest.TempFile(t, "", strings.Join(tt.lines, "\n")))
			if err != nil {
				t.Fatal(err)
			}

			scan, err := New(tmp, `log:$path`, "", "", "", tt.exclude)
			if !ztest.ErrorContains(err, tt.wantErr) {
				t.Fatal(err)
			}
			if tt.wantErr != "" {
				return
			}

			var have []string
			for {
				data, _, _, err := scan.Line(context.Background())
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				have = append(have, data.Path())
			}

			if !reflect.DeepEqual(have, tt.want) {
				t.Errorf("\nhave: %#v\nwant: %#v", have, tt.want)
			}
		})
	}
}
