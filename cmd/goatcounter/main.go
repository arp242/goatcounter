package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	_ "time/tzdata"

	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/goatcounter/v2/db/migrate/gomig"
	"zgo.at/zdb"
	"zgo.at/zdb/drivers"
	"zgo.at/zdb/drivers/go-sqlite3"
	_ "zgo.at/zdb/drivers/pq"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zfs"
	"zgo.at/zstd/zruntime"
	"zgo.at/zstd/zslice"
)

func init() {
	errors.Package = "zgo.at/goatcounter/v2"
}

type command func(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error

func main() {
	var (
		f     = zli.NewFlags(os.Args)
		ready = make(chan struct{}, 1)
		stop  = make(chan struct{}, 1)
	)
	cmdMain(f, ready, stop)
}

var mainDone sync.WaitGroup

func cmdMain(f zli.Flags, ready chan<- struct{}, stop chan struct{}) {
	mainDone.Add(1)
	defer mainDone.Done()

	cmd, err := f.ShiftCommand("help", "version", "serve", "import",
		"dashboard", "db", "monitor",
		"saas", "goat")
	if zslice.ContainsAny(f.Args, "-h", "-help", "--help") {
		f.Args = append([]string{cmd}, f.Args...)
		cmd = "help"
	}
	if err != nil && !errors.Is(err, zli.ErrCommandNoneGiven{}) {
		zli.Errorf(usage[""])
		zli.Errorf("%s", err)
		zli.Exit(1)
		return
	}

	var run command
	switch cmd {
	default:
		zli.Errorf(usage[""])
		zli.Errorf("unknown command: %q", cmd)
		zli.Exit(1)
	case "", "help":
		run = cmdHelp
	case "version":
		fmt.Fprintln(zli.Stdout, getVersion())
		zli.Exit(0)
		return

	case "db", "database":
		run = cmdDB
	case "serve":
		run = cmdServe
	case "saas":
		run = cmdSaas
	case "monitor":
		run = cmdMonitor
	case "import":
		run = cmdImport
	case "dashboard":
		// Wrap as this also doubles as an example, and these flags just obscure
		// things.
		run = func(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
			defer func() { ready <- struct{}{} }()
			return cmdDashboard(f)
		}
	case "goat":
		run = func(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
			defer func() { ready <- struct{}{} }()
			fmt.Print(goat[1:])
			return nil
		}

	case "create":
		flags := os.Args[2:]
		for i, ff := range flags {
			if ff == "-domain" {
				flags[i] = "-vhost"
			}
			if strings.HasPrefix(ff, "-domain=") {
				flags[i] = "-vhost=" + ff[8:]
			}
		}
		fmt.Fprintf(zli.Stderr,
			"The create command is moved to \"goatcounter db create site\"\n\n\t$ goatcounter db create site %s\n",
			strings.Join(flags, " "))
		zli.Exit(5)
		return
	}

	err = run(f, ready, stop)
	if err != nil {
		if !slices.Contains(zlog.Config.Debug, "cli-trace") {
			for {
				var s *errors.StackErr
				if !errors.As(err, &s) {
					break
				}
				err = s.Unwrap()
			}
		}

		c := 1
		var stErr interface {
			Code() int
			Error() string
		}
		if errors.As(err, &stErr) {
			c = stErr.Code()
			if c > 255 { // HTTP error.
				c = 1
			}
		}

		if c == 0 {
			if err.Error() != "" {
				fmt.Fprintln(zli.Stdout, err.Error())
			}
			zli.Exit(0)
		}
		zli.Errorf(err)
		zli.Exit(c)
		return
	}
	zli.Exit(0)
}

func connectDB(connect, dbConn string, migrate []string, create, dev bool) (zdb.DB, context.Context, error) {
	if strings.Contains(connect, "://") && !strings.Contains(connect, "+") {
		connect = strings.Replace(connect, "://", "+", 1)
		zlog.Errorf(`WARNING: the connection string for -db changed from "engine://connectString" to "engine+connectString"; the ://-variant will work for now, but will be removed in a future release`)
	}

	var open, idle int
	if dbConn != "" {
		openS, idleS, ok := strings.Cut(dbConn, ",")
		if !ok {
			return nil, nil, errors.New("-dbconn flag: must be as max_open,max_idle")
		}
		var err error
		open, err = strconv.Atoi(openS)
		if err != nil {
			return nil, nil, fmt.Errorf("-dbconn flag: %w", err)
		}
		idle, err = strconv.Atoi(idleS)
		if err != nil {
			return nil, nil, fmt.Errorf("-dbconn flag: %w", err)
		}
	}

	fsys, err := zfs.EmbedOrDir(goatcounter.DB, "db", dev)
	if err != nil {
		return nil, nil, err
	}

	sqlite3.DefaultHook(goatcounter.SQLiteHook)

	db, err := zdb.Connect(context.Background(), zdb.ConnectOptions{
		Connect:      connect,
		Files:        fsys,
		Migrate:      migrate,
		GoMigrations: gomig.Migrations,
		Create:       create,
		MaxOpenConns: open,
		MaxIdleConns: idle,
		MigrateLog:   func(name string) { zlog.Printf("running migration %q", name) },
	})
	var pErr *zdb.PendingMigrationsError
	if errors.As(err, &pErr) {
		zlog.Errorf("%s; continuing but things may be broken", err)
		err = nil
	}

	// TODO: maybe ask for confirmation here?
	var cErr *drivers.NotExistError
	if errors.As(err, &cErr) {
		if cErr.DB == "" {
			err = fmt.Errorf("%s database at %q exists but is empty.\n"+
				"Add the -createdb flag to create this database if you're sure this is the right location",
				cErr.Driver, connect)
		} else {
			err = fmt.Errorf("%s database at %q doesn't exist.\n"+
				"Add the -createdb flag to create this database if you're sure this is the right location",
				cErr.Driver, cErr.DB)
		}
	}
	if err != nil {
		return nil, nil, err
	}

	// Load languages.
	var c int
	err = db.Get(context.Background(), &c, `select count(*) from languages`)
	// Ignore the error intentionally; not being able to select from the
	// languages table here to populate it (usually because it doesn't exist
	// yet) shouldn't be a fatal error. If there's some other error then the
	// query error will show that one anyway.
	if err == nil && c == 0 {
		langs, err := fs.ReadFile(goatcounter.DB, "db/languages.sql")
		if err != nil {
			return nil, nil, err
		}
		err = db.Exec(context.Background(), string(langs))
		if err != nil {
			return nil, nil, err
		}
	}

	return db, goatcounter.NewContext(db), nil
}

func getVersion() string {
	return fmt.Sprintf("version=%s; go=%s; GOOS=%s; GOARCH=%s; race=%t; cgo=%t",
		goatcounter.Version, runtime.Version(), runtime.GOOS, runtime.GOARCH,
		zruntime.Race, zruntime.CGO)
}
