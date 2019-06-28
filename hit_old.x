type Hits []Hit

func (h *Hits) List(ctx context.Context) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)
	err := db.SelectContext(ctx, h, `select * from hits
		where site=$1 order by created_at desc limit 500;`, site.ID)
	return errors.Wrap(err, "Hits.List")
}

/*
type HitList struct {
	Page  string `db:"page" json:"page"`
	Count int    `db:"count" json:"count"`

	//CreatedAt time.Time `db:"created_at" json:"created_at"`
	//CreatedAt string `db:"created_at" json:"created_at"`
}
*/


/*
func (h HitStats) String() string {
	if len(h) == 0 {
		return "[]"
	}

	var b strings.Builder
	b.WriteString("[")
	for i, st := range h {
		b.WriteString(fmt.Sprintf(`["%s", %d]`, st.CreatedAt, st.Count))
		if len(h) > i+1 {
			b.WriteString(",")
		}
	}
	b.WriteString("]")
	return b.String()
}
*/

/*
func (h *HitList) List(ctx context.Context) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	err := db.SelectContext(ctx, h, `
		select count(*) || " " || path
		from hits
		where site=$1
		group by path
		order by count(*) desc
	`, site.ID)
	return err
}

func (h *HitStats) Hourly(ctx context.Context, days int) (int, error) {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	// Get day relative to user TZ.
	//usertz, err := time.LoadLocation("Pacific/Auckland") // TODO
	usertz, err := time.LoadLocation("Europe/Amsterdam") // TODO
	if err != nil {
		panic(err)
	}
	t := now.New(time.Now().In(usertz)).BeginningOfDay().UTC()
	start := t.Format(time.RFC3339)
	end := t.Add(24 * time.Hour).Format(time.RFC3339)

	err = db.SelectContext(ctx, h, `
		select count(*) as count,
		created_at
		from hits
		where
			site=$1 and
			created_at >= $2 and created_at <= $3
		group by strftime("%H", created_at)
		order by created_at asc
	`, site.ID, start, end)

	hh := *h
	for i := range hh {
		hh[i].CreatedAt = now.New(hh[i].CreatedAt.In(usertz)).BeginningOfHour()
	}

	return 0, err
}

func (h *HitStats) Daily(ctx context.Context, days int) (int, error) {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	err := db.SelectContext(ctx, h, `
		select count(*) as count,
		strftime("%Y-%m-%d", created_at) as created_at
		from hits
		where site=$1
		group by strftime("%Y-%m-%d", created_at)
		order by created_at asc
	`, site.ID)

	return 0, err
}
*/

type RefStats []string

func (r *RefStats) ListPath(ctx context.Context, path string) error {
	db := MustGetDB(ctx)
	site := MustGetSite(ctx)

	err := db.SelectContext(ctx, r, `
		select count(*) || " " || ref
		from hits
		where site=$1 and path=$2
		group by ref
		order by count(*) desc
		limit 50
	`, site.ID, path)
	return errors.Wrap(err, "RefStats.ListPath")
}
