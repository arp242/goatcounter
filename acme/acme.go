package acme

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"sync"
	"time"

	"github.com/pkg/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zlog"
)

var Domains = make(chan string)

var wg sync.WaitGroup

func Run() {
	go func() {
		for {
			go makecert(<-Domains)
		}
	}()
}

func makecert(domain string) {
	wg.Add(1)
	defer wg.Done()

	l := zlog.Module("makecert").Fields(zlog.F{"domain": domain})
	if !cfg.Prod {
		l.Print("skipping on dev")
		return
	}

	if cfg.CertDir == "" {
		l.Errorf("cfg.CertDir is blank")
		return
	}

	// TODO: a Go solution would be better, but this is easier for now.
	out, err := exec.Command("acme.sh",
		"--home", fmt.Sprintf("%s/acme.sh", cfg.CertDir),
		"--webroot", cfg.CertDir,
		"--issue", "-d", domain).CombinedOutput()
	if err != nil {
		l.Fields(zlog.F{"out": string(out)}).Error(errors.Wrap(err, "acme.sh"))
		return
	}

	key, err := ioutil.ReadFile(fmt.Sprintf(
		"%s/acme.sh/%s/%[2]s.key", cfg.CertDir, domain))
	if err != nil {
		l.Error(err)
		return
	}

	chain, err := ioutil.ReadFile(fmt.Sprintf(
		"%s/acme.sh/%s/fullchain.cer", cfg.CertDir, domain))
	if err != nil {
		l.Error(err)
		return
	}

	err = ioutil.WriteFile(
		fmt.Sprintf("%s/pem/%s.pem", cfg.CertDir, domain),
		append(append(key, '\n'), chain...), 0755)
	if err != nil {
		l.Error(err)
		return
	}
}

func Wait() {
	time.Sleep(50 * time.Millisecond)
	wg.Wait()
}
