Here are some benchmarks of the GoatCounter dashboard, just to give an indication
of what to expect with the SQLite and PostgreSQL databases.

These tests were all performed on Linode (SG) with Alpine Linux 3.13 (28 March
2021), PostgreSQL 13.2, and SQLite 3.34.0 using GoatCounter 2.0.0. Your exact
performance may differ based on VPS provider, location, or even individual VPS
in the same data centre. They're just intended as a rough comparison. I used
Linode because that's what I already use.

GoatCounter was run with `goatcounter serve`; no extra flags or proxy. To
simulate real-world loading times I ran the benchmarks from my laptop, rather
than the VPS.

Every configuration is run against SQLite and PostgreSQL with four different
databases:

- 1M pageviews, evenly spread out over a year and 1,000 paths.
- 1M pageviews, evenly spread out over a year and 50,000 paths.
- 10M pageviews, evenly spread out over a year and 1,000 paths.
- 10M pageviews, evenly spread out over a year and 50,000 paths.

The performance depends on two factors: the number of pageviews, and the number
of unique paths you have. For example a site with 10M pageviews spread out over
50 paths will be a lot faster than 1M pageviews spread out over 200,000 paths.

The bencharks were run on the following configurations:

- Nanode 1GB, 1 core, $5/month
- Linode 2GB, 1 core, $10/month
- Linode 4GB, 2 cores, $20/month
- Linode 8GB, 4 cores, $40/month

The results aren't completely consistent; when resizing a Linode it may get
moved to a different physical machine, which may have better/worse performance.
It may also be affected by temporary performance spikes and the like. It is the
nature of these kind of cloud things. I didn't make a lot of effort to re-run
them a day later and/or on other locations and such as it's all a lot of work.
This is okay, as the overview is just intended as a rough rule-of-thumb
overview.


Results
-------

The times are the average of 10 requests accessing the dashboard with the given
date range. After 60 seconds GoatCounter will kill the query; `>60s` means it
timed out.


### SQLite

|                 | 1G/1c | 2G/1c | 4G/2c | 8G/4c |
| --------------  | ----: | ----: | ----: | ----: |
| 1M/1k week      | 1.8s  | 1.4s  | 1.3s  | 0.8s  |
| 1M/1k month     | 5s    | 5.4s  | 3.7s  | 1.7s  |
| 1M/1k quarter   | 12.9s | 15.5s | 10.3s | 4s    |
| 1M/1k year      | >60s  | >60s  | >60s  | 13.6s |
|                 |       |       |       |       |
| 1M/50k week     | 1.9s  | 1.6s  | 1.5s  | 0.9s  |
| 1M/50k month    | 5.4s  | 5.5s  | 4.2s  | 1.9s  |
| 1M/50k quarter  | 15.8s | 16.4s | 11s   | 5s    |
| 1M/50k year     | >60s  | >60s  | >60s  | 15.3s |
|                 |       |       |       |       |
| 10M/1k week     | 19s   | 7s    | 6.5s  | 3.3s  |
| 10M/1k month    | >60s  | >60s  | 19.4s | 8.9s  |
| 10M/1k quarter  | >60s  | >60s  | >60s  | >60s  |
| 10M/1k year     | >60s  | >60s  | >60s  | >60s  |
|                 |       |       |       |       |
| 10M/50k week    | >60s  | 19.3s | 11.1s | 5.9s  |
| 10M/50k month   | >60s  | >60s  | >60s  | 16.8s |
| 10M/50k quarter | >60s  | >60s  | >60s  | >60s  |
| 10M/50k year    | >60s  | >60s  | >60s  | >60s  |


### PostgreSQL

|                 | 1G/1c | 2G/1c | 4G/2c | 8G/4c |
| --------------  | ----: | ----: | ----: | ----: |
| 1M/1k week      | 0.2s  | 0.1s  | 0.6s  | 0.1s  |
| 1M/1k month     | 1.8s  | 0.8s  | 1.2s  | 0.3s  |
| 1M/1k quarter   | 4.2s  | 2.4s  | 3.4s  | 0.9s  |
| 1M/1k year      | 5.3s  | 3.2s  | 3.6s  | 1.5s  |
|                 |       |       |       |       |
| 1M/50k week     | 0.4s  | 0.3s  | 0.5s  | 0.3s  |
| 1M/50k month    | 1.7s  | 0.9s  | 1.7s  | 0.4s  |
| 1M/50k quarter  | 4.8s  | 2.9s  | 3.8s  | 1.2s  |
| 1M/50k year     | 6.7s  | 4.5s  | 4.9s  | 2s    |
|                 |       |       |       |       |
| 10M/1k week     | 12.4s | 2.4s  | 2.1s  | 1.9s  |
| 10M/1k month    | >60s  | 9.3s  | 2.7s  | 1.6s  |
| 10M/1k quarter  | >60s  | 10.9s | 2.4s  | 0.9s  |
| 10M/1k year     | >60s  | 10.6s | 2.4s  | 0.9s  |
|                 |       |       |       |       |
| 10M/50k week    | >60s  | >60s  | 3.9s  | 2s    |
| 10M/50k month   | >60s  | >60s  | 10.5s | 2.4s  |
| 10M/50k quarter | >60s  | >60s  | 11s   | 2s    |
| 10M/50k year    | >60s  | >60s  | 11s   | 1.9s  |


The take-away is that PostgreSQL is a lot faster; but I've also spent more time
optimizing PostgreSQL so there may be something to win here.

Many smaller sites will have much fewer than 1 million pageview and 1,000 paths
though. If you're running GoatCounter on a small website then the cheapest
$5/month should be enough.

The amount of pageviews it can record isn't benchmarked here, but should be
hundreds/second even on smaller machines. You'll likely hit in to performance
problems on the dashboard before you hit problems there.

You can find the "raw" output from `hey` over here:
https://gist.github.com/arp242/ae9d409e47dfe1021ab0b4ff3e5faba7


Benchmark details
-----------------

You can use `./cmd/gcbench` to set up a database. The help on that for some
details.

I used [hey][hey] to run the actual benchmarks with a simple script:

    #!/bin/sh
    #
    # https://github.com/arp242/goatcounter/blob/master/docs/benchmark.markdown
    # Requires "hey": https://github.com/rakyll/hey

    url=https://gcbench.arp242.net
    start=$(date -d @$(( $(date +%s) - 86400 * 5))  +%Y-%m-%d)  # Created DB 5 days ago
    run() {
        send="$url?period-end=$start&period-start=$(date -d @$(( $(date +%s) - 86400 * $1)) +%Y-%m-%d)"
        echo "$send" | tee "gcbench/$1"
        ./hey -c 1 -n 10 "$send" | tee -a "gcbench/$1"
    }

    mkdir -p gcbench
    run 7
    run 30
    run 90
    run 182
    run 365


[hey]: https://github.com/rakyll/hey


### PostgreSQL

PostgreSQL 13.2 was run with the following configuration:

    max_wal_size              = 4GB     # Default: 1G
    min_wal_size              = 256MB   # Default: 80M
    random_page_cost          = 0.5     # Better for SSDs.
    effective_io_concurrency  = 128     # Better for SSDs.
    default_statistics_target = 1000    # Sample more in ANALYZE; Default: 100

And the following were tweaked based on the Linode instance size:

Nanode 1GB/1 core   $5/month

    max_parallel_workers_per_gather  = 1       # Number of cores.
    max_parallel_maintenance_workers = 1       # Half the cores.
    shared_buffers                   = 256MB   # ~25% of RAM
    effective_cache_size             = 768MB   # ~75% of RAM
    maintenance_work_mem             = 52MB    # ~5% of RAM
    work_mem                         = 21MB    # ~2% of RAM


Linode 2GB/1 core  $10/month

    max_parallel_workers_per_gather  = 1       # Number of cores.
    max_parallel_maintenance_workers = 1       # Half the cores.
    shared_buffers                   = 512MB   # ~25% of RAM
    effective_cache_size             = 1536MB  # ~75% of RAM
    maintenance_work_mem             = 100MB   # ~5% of RAM
    work_mem                         = 40MB    # ~2% of RAM


Linode 4GB/2 cores  $20/month

    max_parallel_workers_per_gather  = 2       # Number of cores.
    max_parallel_maintenance_workers = 1       # Half the cores.
    shared_buffers                   = 1GB     # ~25% of RAM
    effective_cache_size             = 3GB     # ~75% of RAM
    maintenance_work_mem             = 205MB   # ~5% of RAM
    work_mem                         = 82MB    # ~2% of RAM

Linode 8GB/4 cores  $40/month

    max_parallel_workers_per_gather  = 4       # Number of cores.
    max_parallel_maintenance_workers = 2       # Half the cores.
    shared_buffers                   = 2GB     # ~25% of RAM
    effective_cache_size             = 6GB     # ~75% of RAM
    maintenance_work_mem             = 410MB   # ~5% of RAM
    work_mem                         = 164MB   # ~2% of RAM
