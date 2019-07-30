package smail

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"zgo.at/goatcounter/cfg"
	"zgo.at/zlog"
)

var reNL = regexp.MustCompile(`(\r\n){2,}`)

// Send an email.
func Send(subject string, from mail.Address, to []mail.Address, body string) error {
	msg := Format(subject, from, to, body)

	if cfg.SMTP == "" {
		l := strings.Repeat("═", 50)
		fmt.Println("╔═══ EMAIL " + l + "\n║ " +
			strings.Replace(strings.TrimSpace(string(msg)), "\r\n", "\r\n║ ", -1) +
			"\n╚══════════" + l + "\n")
		return nil
	}

	srv, err := url.Parse(cfg.SMTP)
	if err != nil {
		return err
	}

	user := srv.User.Username()
	pw, _ := srv.User.Password()
	host := srv.Host
	if h, _, err := net.SplitHostPort(srv.Host); err == nil {
		host = h
	}

	toList := make([]string, len(to))
	for i := range to {
		toList[i] = to[i].Address
	}

	go func() {
		var auth smtp.Auth
		if user != "" {
			auth = smtp.PlainAuth("", user, pw, host)
		}

		err := smtp.SendMail(srv.Host, auth, from.Address, toList, msg)
		if err != nil {
			zlog.Fields(zlog.F{
				"host": srv.Host,
				"from": from,
				"to":   "toList",
			}).Error(errors.Wrap(err, "smtp.SendMail"))
		}
	}()
	return nil
}

// Format a message.
func Format(subject string, from mail.Address, to []mail.Address, body string) []byte {
	var msg strings.Builder
	t := time.Now()

	fmt.Fprintf(&msg, "From: %s\r\n", from.String())

	tos := make([]string, len(to))
	for i := range to {
		tos[i] = to[i].String()
	}
	fmt.Fprintf(&msg, "To: %s\r\n", strings.Join(tos, ","))

	fmt.Fprintf(&msg, "Date: %s\r\n", t.Format(time.RFC1123Z))
	fmt.Fprintf(&msg, "Content-Type: text/plain;charset=utf-8\r\n")
	fmt.Fprintf(&msg, "Content-Transfer-Encoding: quoted-printable\r\n")
	fmt.Fprintf(&msg, "Message-ID: <login-%s-%s@arp242.net>\r\n", h(body), r())
	fmt.Fprintf(&msg, "Subject: %s\r\n", e(subject))
	msg.WriteString("\r\n")

	w := quotedprintable.NewWriter(&msg)
	w.Write([]byte(body))
	w.Close()

	return []byte(msg.String())
}

func e(s string) string {
	return mime.QEncoding.Encode("utf-8", reNL.ReplaceAllString(s, ""))
}

func h(s string) string {
	w := fnv.New64a()
	w.Write([]byte(s))
	return strconv.FormatUint(w.Sum64(), 36)
}

var max = big.NewInt(0).SetUint64(18446744073709551615)

func r() string {
	n, _ := rand.Int(rand.Reader, max)
	return strconv.FormatUint(n.Uint64(), 36)
}
