package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"time"

	"github.com/chromedp/chromedp"
)

func setup(script string) (context.Context, string, func()) {
	ctx, cancel := chromedp.NewContext(context.Background(),
		chromedp.WithLogf(log.Printf))
	ctx, cancel2 := context.WithTimeout(ctx, 10*time.Second)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<!DOCTYPE html>
			<html><head><title>Test</title></head><body><p>Hello</p>
			<script data-goatcounter="http://test.goatcounter.localhost:31874/count"
					async src="//static.goatcounter.localhost:31874/count.js"></script>
			</body></html>`)
	}))

	tmpdir, err := ioutil.TempDir("", "goatcounter")
	if err != nil {
		log.Fatal(err)
	}
	tmpdb := tmpdir + "/goatcounter.sqlite3"
	defer os.RemoveAll(tmpdir)

	// TODO: persist stuff faster in cron, need to wait too long now.
	// TODO: run main directly?
	// TODO: create initial site.
	gc := exec.CommandContext(ctx, "goatcounter", "serve",
		"-db", "sqlite://"+tmpdb,
		"-debug", "all",
		"-tls", "none",
		"-listen", "localhost:31874")
	gc.Stdout = os.Stdout
	gc.Stderr = os.Stderr
	gc.Start()

	return ctx, ts.URL, func() {
		cancel()
		cancel2()
		ts.Close()
		os.RemoveAll(tmpdir)
	}
}

func main() {
	ctx, addr, cancel := setup("")
	defer cancel()

	err := chromedp.Run(ctx, chromedp.Navigate(addr), chromedp.WaitReady(`script`))
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(12 * time.Second) // TODO: wait for console output.
}
