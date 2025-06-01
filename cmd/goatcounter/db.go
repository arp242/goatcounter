package main

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"zgo.at/errors"
	"zgo.at/goatcounter/v2"
	"zgo.at/guru"
	"zgo.at/z18n"
	"zgo.at/zdb"
	"zgo.at/zdb/drivers"
	"zgo.at/zli"
	"zgo.at/zlog"
	"zgo.at/zstd/zcrypto"
	"zgo.at/zstd/zint"
	"zgo.at/zstd/zstring"
	"zgo.at/zstd/ztype"
	"zgo.at/zvalidate"
)

const helpDB = `
The db command manages the GoatCounter database.

Some common examples:

    Create a new site:

        $ goatcounter db create site -vhost stats.example.com -user.email martin@example.com

        You will log in with the -email; -vhost is where your GoatCounter site
        will be accessible from.

    Create a new API key:

        $ goatcounter db create apitoken -name 'My token' -user 1 -perm count

    Run database migrations:

        $ goatcounter db migrate all

` + helpDBCommands + `

Flags accepted by all commands:

  -db          Database connection: "sqlite+<file>" or "postgres+<connect>"
               See "goatcounter help db" for detailed documentation. Default:
               sqlite+./db/goatcounter.sqlite3 if that database file exists, or
               sqlite+./goatcounter-data/db.sqlite3 if it doesn't.

  -createdb    Create the database if it doesn't exist yet.

  -debug       Modules to debug, comma-separated or 'all' for all modules.
               See "goatcounter help debug" for a list of modules.

show command:

    -find       Object to find; you can always use the numeric ID column (e.g. 1),
                but also a friendlier name:

                    site      vhost ("stats.example.com").
                    user      email ("user@example.com").
                    apitoken  Token secret (5a1f...).

    -format     Format to print, accepted values:

                    table     ASCII table, one row per line.
                    vertical  Vertical table, one column per line (default).
                    csv       CSV (includes header).
                    json      JSON, as an array of objects.
                    html      HTML table.

delete command:

    -find       As documented in show

    -force      Force deletion.

                    site       If there are sites linked to this site then it
                               will also delete all those sites.
                    user       Force deletion even if this is the last admin.
                    apitoken   No effect.

create and update commands:

    The create and update commands accept a set of flags with column values. You
    can't set all columns, just the useful ones for regular management. The
    flags for create and update are identical, except that "update" also needs a
    -find flag; this is documented above in the show command.

    You may need to restart GoatCounter for some changes to take effect due to
    caching.

    You can add multiple -find flags to update multiple rows.

    Flags marked with * are required for create; for update only the flags that
    are given are updated.

    Flags for "site":

        -vhost*     Domain to host this site at (e.g. "stats.example.com"). The
                    site will be available on this domain only, so
                    "stats.example.com" won't be available on "localhost".

        -link*      Link to this site; the site will use the same users, copies
                    this site's settings on creation, and will be listed in the
                    top navigation
                    Can be as ID ("1") or vhost ("stats.example.com").

        Only or "create", as a convenience to create a new user:

            -user.email*      Your email address. Will be required to login.
                              This can not be set if -link is also set (since
                              that will re-use the same users as the linked
                              site).

            -user.password*   Password to log in; will be asked interactively if omitted.

    Flags for "user":

        -site*      Site to add the user to. Same format as -find for site.

        -email*     Email address; required to log in.

        -access*    Access to give this user:

                        readonly    Can't change any settings.
                        settings    Can change settings, except site/user
                                    management.
                        admin       Full access.
                        superuser   Full access, including the "server
                                    management" page.

        -password   Password; will be asked interactively if omitted.

    Flags for "apitoken":

        -user*      User to create API key for. Same format as -find for user.

        -name       API key name.

        -perm*      Comma-separated list of permissions to assign; possible
                    values:

                        count        Allow recording pageviews with /v0/count
                        export       Allow creating exports.
                        site_read    Reading site information.
                        site_create  Creating new sites.
                        site_update  Updating existing sites.

migrate command:

    Run or print database migrations.

        -dev        Load migrations from filesystem, rather than using the
                    migrations compiled in the binary.

        -test       Rollback migration after running the migrations instead of
                    committing it. Useful to test if migrations will run
                    correctly without actually altering the database.

        -show       Only show the SQL it would execute, but don't run anything.

    Positional arguments are names of the migration, either as just the name
    ("2020-01-05-2-x") or as the file path ("./db/migrate/2020-01-05-2-x.sql").

    Special values:

        all         Run all pending migrations.
        pending     Show pending migrations but do not run anything. Exits with 1 if
                    there are pending migrations, or 0 if there aren't.
        list        List all migrations; pending migrations are prefixed with
                    "pending: ". Always exits with 0.

    Note: you can also use -automigrate flag for the serve command to run migrations
    on startup.

newdb command:

    Create a new database. This is the same what "goatcounter serve" or
    "goatcounter db -createdb [command]" do if no database exists yet.

    Exits with 0 if the database was already created, 2 if the database already
    exists (integrity isn't checked, just existence), or 1 on any other error.

schema-sqlite and schema-pgsql commands:

    Print the compiled-in database schema for SQLite or PostgreSQL, in case you
    want to create the database manually from the schema.

test command:

    Test if the database exists; exits with 0 on success, 2 if the database
    doesn't exist, and 1 on any other error.

    This is useful for setting up new databases in scripts if you don't want to
    use the default database creation; e.g.:

        goatcounter db test -db [..]
        if [ $? -eq 2 ]; then
            createdb goatcounter
            goatcounter db schema-pgsql | psql goatcounter
        fi

query command:

    Run a query against the database, this is unrestricted and can modify/delete
    anything, so use with care. Can be useful in cases where you don't have a
    psql or sqlite3 CLI available.

    Only runs one query, unless -format=exec is given.

    -format         Format to print, accepted values:

                        table     ASCII table, one row per line (default).
                        vertical  Vertical table, one column per line.
                        csv       CSV (includes header).
                        json      JSON, as an array of objects.
                        html      HTML table.
                        exec      Execute the query but don't return result.
                                  Mostly useful for multiple "create table"
                                  queries.

    There are a few special values to get some data:

        screensize      Get some aggregate data about screen sizes.
        ua              List all User-Agent headers, but not bots.
        bots            List all User-Agent headers that are a "bot".
        unknown-ua      List all User-Agent headers that do not have full
                        browser/system associated with them.

Detailed documentation on the -db flag:

    GoatCounter can use SQLite and PostgreSQL. All commands accept the -db flag
    to customize the database connection string.

    You can select a database engine by using "sqlite+[..]" for SQLite, or
    "postgresql+[..]" (or "postgres+[..]") for PostgreSQL.

    There are no plans to support other database engines such as MySQL/MariaDB.

    SQLite should work fine for most smaller site like blogs and such, but for
    more serious usage PostgreSQL is recommended. Some basic benchmarks
    comparing the two can be found here:
    https://github.com/arp242/goatcounter/blob/master/docs/benchmark.md

    The database is automatically created for the "serve" command, but you need
    to add -createdb to any other commands to create the database. This is to
    prevent accidentally operating on the wrong (new) database.

SQLite notes:

    This is the default database engine as it has no dependencies, and for most
    small to medium usage it should be more than fast enough.

    The SQLite connection string is usually just a filename, optionally prefixed
    with "file:". Parameters can be added as a URL query string after a ?:

        -db 'sqlite+mydb.sqlite?param=value&other=value'

    See the go-sqlite3 documentation for a list of supported parameters:
    https://github.com/mattn/go-sqlite3/#connection-string

    A few parameters are different from the SQLite defaults:

        _journal_mode=wal          Almost always faster with better concurrency,
                                   with little drawbacks for most use cases.
        _busy_timeout=200          Wait 200ms for locks instead of immediately
                                   throwing an error.
        _cache_size=-20000         20M cache size, instead of 2M. Can be a
                                   significant performance improvement.

    But you can change them if you wish; for example to use the SQLite defaults:

        -db 'sqlite+mydb.sqlite?_journal_mode=delete&_busy_timeout=0&_cache_size=-2000'

PostgreSQL notes:

    PostgreSQL provides better performance for large instances. If you have
    millions of pageviews then PostgreSQL is probably a better choice.

    The PostgreSQL connection string can either be as "key=value" or as an URL;
    the following are identical:

        -db 'postgresql+user=pqgotest dbname=pqgotest sslmode=verify-full'
        -db 'postgresql+postgres://pqgotest:password@localhost/pqgotest?sslmode=verify-full'

    See the pq documentation for a list of supported parameters:
    https://pkg.go.dev/github.com/lib/pq?tab=doc#hdr-Connection_String_Parameters

    You can also use the standard PG* environment variables:

        PGDATABASE=goatcounter PGHOST=/var/run goatcounter -db 'postgresql'

    You may want to consider lowering the "seq_page_cost" parameter; the query
    planner tends to prefer seq scans instead of index scans for some operations
    with the default of 4, which is much slower. I found that 1.1 is a fairly
    good setting, you can set it in your postgresql.conf file, or just for one
    database with:

        alter database goatcounter set seq_page_cost=1.1

Converting from SQLite to PostgreSQL:

    You can use pgloader (https://pgloader.io) to convert from a SQLite to
    PostgreSQL database; to do this you must first create the PostgreSQL schema
    manually (pgloader doesn't create it properly), remove the data from the
    locations and languages tables as they will conflict.

    For example:

        # Create new PostgreSQL database
        $ createdb --owner goatcounter
        $ goatcounter db migrate all -createdb -db postgresql+dbname=goatcounter

        # Remove data from languages and locations tables
        $ psql goatcounter -c 'delete from locations; delete from languages;'

        # Convert data with pgloader
        $ pgloader --with 'create no tables' ./goatcounter-data/db.sqlite3 postgresql:///goatcounter
`

const helpDBCommands = `List of commands:

     create [table]     Create a new row.
     update [table]     Update a row.
     delete [table]     Delete a row.
     show   [table]     Show a row.

                        Valid tables are "site", "user", and "apitoken".

     newdb              Create a new database.
     migrate            Run or view database migrations.
     schema-sqlite      Print the SQLite schema.
     schema-pgsql       Print the PostgreSQL schema.
     test               Test if the database exists.
     query              Run a query.`

const helpDBShort = "\n" + helpDBCommands + `

Use "goatcounter help db" for the full documentation.`

type (
	findMany interface {
		Find(context.Context, []string) error
		IDs() []int64
		Delete(context.Context, bool) error
	}
	stringFlag interface {
		String() string
		Set() bool
	}
)

func cmdDB(f zli.Flags, ready chan<- struct{}, stop chan struct{}) error {
	defer func() { ready <- struct{}{} }()

	var (
		dbConnect = f.String(defaultDB(), "db").Pointer()
		debug     = f.String("", "debug").Pointer()
		createdb  = f.Bool(false, "createdb").Pointer()
	)

start:
	cmd, err := f.ShiftCommand()
	if err != nil && !errors.Is(err, zli.ErrCommandNoneGiven{}) {
		return err
	}

	switch cmd {
	default:
		// Be forgiving if someone reverses the order of "create" and "site".
		maybeCmd := f.Shift()
		if slices.Contains([]string{"create", "update", "delete", "show"}, maybeCmd) {
			f.Args = append([]string{maybeCmd, cmd}, f.Args...)
			goto start
		}

		return errors.Errorf("unknown command for \"db\": %q\n%s", cmd, helpDBShort)
	case "": //, zli.CommandNoneGiven:
		return errors.New("\"db\" needs a subcommand\n" + helpDBShort)
	case "help":
		zli.WantColor = true
		printHelp(helpDB)
		return nil

	case "schema-sqlite", "schema-pgsql":
		return cmdDBSchema(cmd)
	case "test":
		return cmdDBTest(f, dbConnect, debug, true)
	case "migrate":
		return cmdDBMigrate(f, dbConnect, debug, createdb)
	case "query":
		return cmdDBQuery(f, dbConnect, debug, createdb)
	case "show":
		return cmdDBShow(f, cmd, dbConnect, debug, createdb)
	case "delete":
		return cmdDBDelete(f, cmd, dbConnect, debug, createdb)

	case "create", "update":
		tbl, err := getTable(&f, cmd)
		if err != nil {
			return err
		}

		fun := map[string]func(f zli.Flags, cmd string, dbConnect, debug *string, createdb *bool) error{
			"site":     cmdDBSite,
			"user":     cmdDBUser,
			"apitoken": cmdDBAPIToken,
		}[tbl]
		return fun(f, cmd, dbConnect, debug, createdb)

	case "newdb":
		err := cmdDBTest(f, dbConnect, debug, false)
		if err == nil {
			return guru.Errorf(2, "database at %q already exists", *dbConnect)
		}

		var cErr *drivers.NotExistError
		if !errors.As(err, &cErr) {
			return err
		}

		db, _, err := connectDB(*dbConnect, "", []string{"pending"}, true, false)
		if err != nil {
			return err
		}
		return db.Close()
	}
}

func getTable(f *zli.Flags, cmd string) (string, error) {
	tbl, err := f.ShiftCommand()
	if err != nil && !errors.Is(err, zli.ErrCommandNoneGiven{}) {
		return "", err
	}

	switch tbl {
	default:
		return "", errors.Errorf("unknown table %q\n%s", tbl, helpDBShort)
	case "":
		return "", errors.Errorf("%q commands needs a table name\n%s", cmd, helpDBShort)
	case "help":
		zli.WantColor = true
		printHelp(helpDB)
		return "", guru.New(0, "")

	case "site", "sites":
		return "site", nil
	case "user", "users":
		return "user", nil
	case "apitoken", "apitokens":
		return "apitoken", nil
	}
}

func getFormat(format string) (zdb.DumpArg, error) {
	switch format {
	case "table":
		return 0, nil
	case "vertical":
		return zdb.DumpVertical, nil
	case "csv":
		return zdb.DumpCSV, nil
	case "json":
		return zdb.DumpJSON, nil
	case "html":
		return zdb.DumpHTML, nil
	case "exec":
		return -1, nil
	default:
		return 0, fmt.Errorf("-format: unknown value: %q", format)
	}
}

func cmdDBSchema(cmd string) error {
	d, err := goatcounter.DB.ReadFile("db/schema.gotxt")
	if err != nil {
		return err
	}
	driver := zdb.DialectSQLite
	if cmd == "schema-pgsql" {
		driver = zdb.DialectPostgreSQL
	}
	d, err = zdb.Template(driver, string(d))
	if err != nil {
		return err
	}
	fmt.Fprint(zli.Stdout, string(d))
	return nil
}

func cmdDBTest(f zli.Flags, dbConnect, debug *string, print bool) error {
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil && !errors.As(err, &zli.ErrUnknownEnv{}) {
		return err
	}

	if *dbConnect == "" {
		return errors.New("must add -db flag")
	}
	zlog.Config.SetDebug(*debug)
	db, err := zdb.Connect(context.Background(), zdb.ConnectOptions{Connect: *dbConnect})
	if err != nil {
		return err
	}
	defer db.Close()

	ctx := zdb.WithDB(context.Background(), db)

	info, err := db.Info(ctx)
	if err != nil {
		return err
	}

	var i int
	err = db.Get(ctx, &i, `select 1 from version`)
	if err != nil {
		return fmt.Errorf("select 1 from version: %w", err)
	}
	if print {
		fmt.Fprintf(zli.Stdout, "DB at %q seems okay; %s version %s\n",
			*dbConnect, info.DriverName, info.Version)
	}
	return nil
}

func cmdDBQuery(f zli.Flags, dbConnect, debug *string, createdb *bool) error {
	var (
		format = f.String("table", "format")
	)
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil && !errors.As(err, &zli.ErrUnknownEnv{}) {
		return err
	}

	query, err := zli.InputOrArgs(f.Args, " ", false)
	if err != nil {
		return err
	}
	if len(query) == 0 || query[0] == "" {
		return errors.New("need a query")
	}

	zlog.Config.SetDebug(*debug)

	db, ctx, err := connectDB(*dbConnect, "", nil, *createdb, false)
	if err != nil {
		return err
	}
	defer db.Close()

	q, _, err := zdb.Load(db, "db.query."+query[0]+".sql")
	if err != nil {
		q = strings.Join(query, " ")
	}

	dump, err := getFormat(format.String())
	if err != nil {
		return err
	}

	if dump == -1 {
		return zdb.Exec(ctx, q)
	}
	zdb.Dump(ctx, zli.Stdout, q, dump)
	return nil
}

func getManyFinder(ctx context.Context, f *zli.Flags, cmd string, find []string) (findMany, string, error) {
	if len(find) == 0 {
		return nil, "", errors.New("need at least on -find flag")
	}

	tbl, err := getTable(f, cmd)
	if err != nil {
		return nil, "", err
	}

	finder := map[string]findMany{
		"site":     &goatcounter.Sites{},
		"user":     &goatcounter.Users{},
		"apitoken": &goatcounter.APITokens{},
	}[tbl]

	err = finder.Find(ctx, find)
	return finder, tbl, err
}

func dbParseFlag(f zli.Flags, dbConnect, debug *string, createdb *bool) (zdb.DB, context.Context, error) {
	if err := f.Parse(zli.FromEnv("GOATCOUNTER")); err != nil && !errors.As(err, &zli.ErrUnknownEnv{}) {
		return nil, nil, err
	}
	zlog.Config.SetDebug(*debug)

	db, _, err := connectDB(*dbConnect, "", []string{"pending"}, *createdb, false)
	if err != nil {
		return nil, nil, err
	}

	ctx := goatcounter.NewContext(db)
	ctx = z18n.With(ctx, z18n.NewBundle(language.English).Locale("en"))
	return db, ctx, nil
}

func cmdDBShow(f zli.Flags, cmd string, dbConnect, debug *string, createdb *bool) error {
	var (
		find   = f.StringList(nil, "find")
		format = f.String("vertical", "format")
	)
	db, ctx, err := dbParseFlag(f, dbConnect, debug, createdb)
	if err != nil {
		return err
	}
	defer db.Close()

	finder, tbl, err := getManyFinder(ctx, &f, cmd, find.Strings())
	if err != nil {
		return err
	}

	q := map[string]string{
		"site":     "sites where site_id",
		"user":     "users where user_id",
		"apitoken": "api_tokens where api_token_id",
	}[tbl]

	ids := finder.IDs()
	if len(ids) == 0 {
		return errors.New("nothing found")
	}

	dump, err := getFormat(format.String())
	if err != nil {
		return err
	}

	zdb.Dump(ctx, zli.Stdout, `select * from `+q+` in (?)`, ids, dump)
	return nil
}

func cmdDBDelete(f zli.Flags, cmd string, dbConnect, debug *string, createdb *bool) error {
	var (
		find  = f.StringList(nil, "find")
		force = f.Bool(false, "force").Pointer()
	)
	db, ctx, err := dbParseFlag(f, dbConnect, debug, createdb)
	if err != nil {
		return err
	}
	defer db.Close()

	finder, _, err := getManyFinder(ctx, &f, cmd, find.Strings())
	if err != nil {
		return err
	}

	return finder.Delete(ctx, *force)
}

func cmdDBSite(f zli.Flags, cmd string, dbConnect, debug *string, createdb *bool) error {
	// TODO(depr): The second values are for compat with <2.0
	var (
		vhost = f.String("", "vhost", "domain")
		link  = f.String("", "link", "parent")
		find  *[]string
		email stringFlag
		pwd   stringFlag
	)
	if cmd == "update" {
		find = f.StringList(nil, "find").Pointer()
	}
	if cmd == "create" {
		email = f.String("", "user.email", "email")
		pwd = f.String("", "user.password", "password")
	}
	db, ctx, err := dbParseFlag(f, dbConnect, debug, createdb)
	if err != nil {
		return err
	}
	defer db.Close()

	if link.Set() && email != nil && email.Set() {
		return errors.New("can't set both -link and -user.email")
	}

	if cmd == "create" {
		return cmdDBSiteCreate(ctx, vhost.String(), email.String(), link.String(), pwd.String())
	}
	return cmdDBSiteUpdate(ctx, *find, vhost, link)
}

func cmdDBSiteCreate(ctx context.Context, vhost, email, link, pwd string) error {
	v := zvalidate.New()
	v.Required("-vhost", vhost)
	v.Domain("-vhost", vhost)
	if link == "" {
		v.Required("-user.email", email)
		v.Email("-user.email", email)
	}
	if v.HasErrors() {
		return v
	}

	err := (&goatcounter.Site{}).ByHost(ctx, vhost)
	if err == nil {
		return fmt.Errorf("there is already a site for the host %q", vhost)
	}

	var account goatcounter.Site
	if link != "" {
		account, err = findParent(ctx, link)
		if err != nil {
			return nil
		}
	}

	if link == "" && pwd == "" {
		pwd, err = zli.AskPassword(8)
		if err != nil {
			return err
		}
	}

	return zdb.TX(ctx, func(ctx context.Context) error {
		s := goatcounter.Site{
			Code:  "serve-" + zcrypto.Secret64(),
			Cname: &vhost,
		}
		if account.ID > 0 {
			s.Parent, s.Settings, s.UserDefaults = &account.ID, account.Settings, account.UserDefaults
		}
		err := s.Insert(ctx)
		if err != nil {
			return err
		}

		if link == "" { // Create user as well.
			err = (&goatcounter.User{
				Site:          s.ID,
				Email:         email,
				Password:      []byte(pwd),
				EmailVerified: true,
				Settings:      s.UserDefaults,
				Access:        goatcounter.UserAccesses{"all": goatcounter.AccessSuperuser},
			}).Insert(ctx, false)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func cmdDBSiteUpdate(ctx context.Context, find []string,
	vhost, link stringFlag,
) error {

	v := zvalidate.New()
	v.Required("-find", find)
	v.Domain("-vhost", vhost.String())
	if v.HasErrors() {
		return v
	}

	var sites goatcounter.Sites
	err := sites.Find(ctx, find)
	if err != nil {
		return err
	}

	return zdb.TX(ctx, func(ctx context.Context) error {
		for _, s := range sites {
			if link.Set() {
				ps, err := findParent(ctx, link.String())
				if err != nil {
					return err
				}

				err = s.UpdateParent(goatcounter.WithSite(ctx, &ps), &ps.ID)
				if err != nil {
					return err
				}
			}

			if vhost.Set() {
				s.Cname = ztype.Ptr(vhost.String())
				err := s.Update(ctx)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func findParent(ctx context.Context, link string) (goatcounter.Site, error) {
	var s goatcounter.Site
	err := s.Find(ctx, link)
	if err != nil {
		return s, err
	}
	if s.Parent != nil {
		err = s.ByID(ctx, *s.Parent)
	}
	return s, err
}

func cmdDBUser(f zli.Flags, cmd string, dbConnect, debug *string, createdb *bool) error {
	var (
		site   = f.String("", "site")
		email  = f.String("", "email")
		access = f.String("", "access")
		pwd    = f.String("", "password")
		find   *[]string
	)
	if cmd == "update" {
		find = f.StringList(nil, "find").Pointer()
	}
	db, ctx, err := dbParseFlag(f, dbConnect, debug, createdb)
	if err != nil {
		return err
	}
	defer db.Close()

	if cmd == "create" {
		return cmdDBUserCreate(ctx, site.String(), email.String(), access.String(), pwd.String())
	}
	return cmdDBUserUpdate(ctx, *find, site, email, access, pwd)
}

func cmdDBUserCreate(ctx context.Context,
	findSite, email, access, pwd string,
) error {

	v := zvalidate.New()
	v.Required("-site", findSite)
	v.Required("-email", email)
	v.Required("-access", access)
	v.Include("-access", access, []string{"readonly", "settings", "admin", "superuser"})
	if v.HasErrors() {
		return v
	}

	var site goatcounter.Site
	err := site.Find(ctx, findSite)
	if err != nil {
		return err
	}

	if pwd == "" {
		pwd, err = zli.AskPassword(8)
		if err != nil {
			return err
		}
	}

	return (&goatcounter.User{
		Site:     site.ID,
		Email:    email,
		Password: []byte(pwd),
		Settings: site.UserDefaults,
		Access:   getAccess(access),
	}).Insert(ctx, false)
}

func cmdDBUserUpdate(ctx context.Context, find []string,
	site, email, access, pwd stringFlag,
) error {

	v := zvalidate.New()
	v.Required("-find", find)
	v.Email("-email", email.String())
	if v.HasErrors() {
		return v
	}

	var users goatcounter.Users
	err := users.Find(ctx, find)
	if err != nil {
		return err
	}

	return zdb.TX(ctx, func(ctx context.Context) error {
		for _, u := range users {
			ctx = goatcounter.WithSite(ctx, &goatcounter.Site{ID: u.Site})

			if email.Set() {
				u.Email = email.String()
			}
			if access.Set() {
				u.Access = getAccess(access.String())
			}

			if email.Set() || access.Set() {
				err := u.Update(ctx, true)
				if err != nil {
					return err
				}
			}

			if site.Set() {
				var s goatcounter.Site
				err := s.Find(ctx, site.String())
				if err != nil {
					return err
				}

				u.Site = s.ID
				err = u.UpdateSite(ctx)
				if err != nil {
					return err
				}
			}

			if pwd.Set() {
				err := u.UpdatePassword(ctx, pwd.String())
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func getAccess(a string) goatcounter.UserAccesses {
	return goatcounter.UserAccesses{
		"all": map[string]goatcounter.UserAccess{
			"readonly":  goatcounter.AccessReadOnly,
			"settings":  goatcounter.AccessSettings,
			"admin":     goatcounter.AccessAdmin,
			"superuser": goatcounter.AccessSuperuser,
		}[a],
	}
}

func cmdDBAPIToken(f zli.Flags, cmd string, dbConnect, debug *string, createdb *bool) error {
	var (
		user = f.String("", "user")
		name = f.String("", "name")
		perm = f.String("", "perm")
		find *[]string
	)
	if cmd == "update" {
		find = f.StringList(nil, "find").Pointer()
	}
	db, ctx, err := dbParseFlag(f, dbConnect, debug, createdb)
	if err != nil {
		return err
	}
	defer db.Close()

	if cmd == "create" {
		return cmdDBAPITokenCreate(ctx, user.String(), perm.String(), name.String())
	}
	return cmdDBAPITokenUpdate(ctx, *find, name, perm)
}

func cmdDBAPITokenCreate(ctx context.Context,
	findUser, permFlag, name string,
) error {

	v := zvalidate.New()
	v.Required("-user", findUser)
	v.Required("-perm", permFlag)
	if v.HasErrors() {
		return v
	}

	var siteID int64
	findUserID, _ := strconv.ParseInt(findUser, 10, 64)
	err := zdb.Get(ctx, &siteID,
		`select site_id from users where user_id = $1 or email = $2`,
		findUserID, findUser)
	if err != nil {
		return err
	}
	ctx = goatcounter.WithSite(ctx, &goatcounter.Site{ID: siteID})

	var user goatcounter.User
	err = user.Find(ctx, findUser)
	if err != nil {
		return err
	}
	ctx = goatcounter.WithUser(ctx, &user)

	perm, err := getPerm(permFlag)
	if err != nil {
		return err
	}

	return (&goatcounter.APIToken{
		SiteID:      user.Site,
		UserID:      user.ID,
		Name:        name,
		Permissions: perm,
	}).Insert(ctx)
}

func cmdDBAPITokenUpdate(ctx context.Context, find []string,
	name, perm stringFlag,
) error {

	v := zvalidate.New()
	v.Required("-find", find)
	if v.HasErrors() {
		return v
	}

	var tokens goatcounter.APITokens
	err := tokens.Find(ctx, find)
	if err != nil {
		return err
	}

	return zdb.TX(ctx, func(ctx context.Context) error {
		for _, t := range tokens {
			ctx = goatcounter.WithSite(ctx, &goatcounter.Site{ID: t.SiteID})
			ctx = goatcounter.WithUser(ctx, &goatcounter.User{ID: t.UserID})

			if name.Set() {
				t.Name = name.String()
			}
			if perm.Set() {
				p, err := getPerm(perm.String())
				if err != nil {
					return err
				}
				t.Permissions = p
			}

			err := t.Update(ctx)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func getPerm(permFlag string) (zint.Bitflag64, error) {
	var perm zint.Bitflag64
	for _, p := range zstring.Fields(permFlag, ",") {
		pp, ok := map[string]zint.Bitflag64{
			"count":       goatcounter.APIPermCount,
			"export":      goatcounter.APIPermExport,
			"site_read":   goatcounter.APIPermSiteRead,
			"site_create": goatcounter.APIPermSiteCreate,
			"site_update": goatcounter.APIPermSiteUpdate,
		}[p]
		if !ok {
			return 0, fmt.Errorf("-perm: invalid value %q", p)
		}
		perm |= pp
	}
	return perm, nil
}
