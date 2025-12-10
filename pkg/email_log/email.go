package email_log

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"zgo.at/blackmail"
	"zgo.at/slog_align"
	"zgo.at/zstd/zstring"
)

type Email struct {
	m        blackmail.Mailer
	from, to string
	lvl      slog.Level
}

func New(mailer blackmail.Mailer, lvl slog.Level, from, to string) Email {
	return Email{m: mailer, from: from, to: to, lvl: lvl}
}

func (e Email) Enabled(ctx context.Context, l slog.Level) bool {
	return l >= e.lvl
}
func (e Email) WithAttrs(attrs []slog.Attr) slog.Handler {
	return e
}
func (e Email) WithGroup(name string) slog.Handler {
	return e
}
func (e Email) Handle(ctx context.Context, r slog.Record) error {
	// Format message with slog_align.
	buf := new(bytes.Buffer)
	h := slog_align.NewAlignedHandler(buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == "module" || a.Key == "_err" {
				return slog.Attr{}
			}
			return a
		},
	})
	h.SetTimeFormat("Jan _2 15:04:05 ")
	h.SetColor(false)
	h.SetInlineLocation(false)
	h.Handle(context.Background(), r)
	msg := buf.String()

	// Silence spurious errors from some bot.
	if strings.Contains(msg, `ReferenceError: "Pikaday" is not defined.`) &&
		strings.Contains(msg, `Mozilla/5.0 (Windows NT 6.1; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.61 Safari/537.36`) {
		return nil
	}
	// Don't need to send notifications for these
	if strings.Contains(msg, `pq: canceling statement due to user request`) ||
		strings.Contains(msg, `write: broken pipe`) ||
		strings.Contains(msg, `write: connection reset by peer`) ||
		strings.Contains(msg, ": context canceled") {
		return nil
	}

	subject := zstring.GetLine(msg, 1)
	if len(subject) > 15 { // Remove date: "Jun  8 00:51:41"
		subject = strings.TrimSpace(subject[15:])
	}
	subject = strings.TrimPrefix(subject, "ERROR")
	subject = strings.TrimLeft(subject, " \t:")

	go func() {
		err := e.m.Send(subject,
			blackmail.From("", e.from),
			blackmail.To(e.to),
			blackmail.BodyText([]byte(msg)))
		if err != nil {
			// Just output to stderr I guess, can't really do much more if
			// sending email fails.
			fmt.Fprintf(os.Stderr, "emailerrors: %s\n", err)
		}
	}()
	return nil
}
