GoatCounter only counts "visits" rather than "pageviews":

- A "pageview" is every time a page is loaded.

- A "visit" is the first time someone loads a page. Someone reloading the page
  or going to another page and then coming back is counted as one visit.

You almost always want to keep track of visits rather than pageviews. Otherwise
someone reloading the page ten times will show up as ten times, which is not
really meaningful.

This can be disabled in the site settings, at `Settings → Data collection →
Sessions`. If it's disabled every pageview counts as a "visit".

Technical details
-----------------
The way visitors are identified is as follows:

1. A sessionHash is created as hash(siteID + User-Agent + IP).

2. Store this in memory as a sessionHash→UUIDv4 map for 8 hours.

3. Store a UUIDv4→seen_paths map (again in memory), so we can count new visits
   for new paths.

4. Use the UUIDv4 in the database and such.

The IP address and User-Agent are never stored to the database or disk, and
there is no conceivable way to trace the random UUID back to this.

It's only stored in memory, which is needed anyway for basic networking to work.

----

Or in pseudo-code:

    session_key    = site_id + user_agent + IP
    count_as_visit = false

    # We've seen this session before.
    if sessions[session_key] and sessions[session_key].newer_than(8_hours)
        # Only count as visit if this session hasn't visited this path yet.
        if not sessions[session_key].seen_path(current_path)
            count_as_visit = true
            add_current_path(sessions[session_key])
        end
    else
        # Generate new session.
        sessions[session_key] = create_random_uuid()
        add_current_path(sessions[session_key])
        count_as_visit = true
    end

    # Store pageview; only the random UUID in sessions[session_key] is stored,
    # and NOT session_key
    store_pageview()

    # Increate counter to make the charts go up.
    if count_as_visit
        increase_counter_in_database()
    end
