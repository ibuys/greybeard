# Greybeard

> Observe, then decide.

An old school monitoring system, built for the modern environment.

## The Story

I haven't been happy with monitoring systems for a long time. Many years ago I managed a small data center in Des Moines, and spent seven years perfecting my Nagios install. Once I understood what Nagios really *was*, I was able to manipulate the configuration to make it do whatever I wanted it to do. It was always an open tab, and it was the source of truth for the state of everything I was responsible for. By the time I left I had years of data available to graph, a database of previous alerts, and a reliable means of deploying and configuring the system. It was great, I was very happy with it. 

Then, in 2014, I started working in the cloud. Suddenly Nagios started to show its age. Nagios wasn't built to be able to handle instances spinning up and down whenever they wanted, or Lambdas running that had no operating system to monitor, at least not one exposed to be able to be monitored. IP addresses, once so vitally important to monitoring, no longer had as much meaning in the cloud. Systems were ephemeral, it was only the service they provided that mattered. So my Nagios system that I liked so much started to fall by the wayside. 

Apparently it wasn't just me who stopped using Nagios. The plugins I relied on to display the historical graphs stopped working, and last I checked they hadn't been updated in 10 years or more. I'm not one to hold on to old technology, so I started exploring what else was out there. 

I stood up a Prometheus server with Grafana, as one does, and integrated it with our systems. Prometheus is an entirely different proposition than Nagios. With Nagios, you run checks, and those checks return the status of whatever is being monitored and optionally some performance data along with it. Prometheus is different. Prometheus scrapes data from a port on a regular basis, and uses that data to make decisions about how it should react. It stores its data in a time series database, and then lets Grafana be the front end. But it also has its own front end, just in case you'd like to use that. And also Grafana has its own monitoring and alerting system that overlaps with the Prometheus alerting system… so take your pick, I guess?

I've seen the pair used to great success, so I know that it's a good system, it just never worked *for me*. We could use it, but I honestly didn't see the benefit of setting up one system I didn't care for over another system that I didn't care for that came already set up with AWS. Which is how we've wound up using CloudWatch for the past several years. 

But, CloudWatch is a massive system, and I've had a hard time keeping it configured the way I like it. I've had issues with dashboards losing access to their data source, and I've never liked how alerts wind up being complicated pipelines through Lambdas. Somehow or another, entirely my fault I'm sure, I get emails with nothing but raw JSON. (Yes, I know its because I didn't parse the JSON before it was piped off to SES. Again, good grief.) So, I've either got a weird scraping system, or I've got a massive machine at the heart of AWS that seems to need a ridiculous amount of configuration to keep working. 

I'm sure that's not fair to either Prometheus or CloudWatch. I'm also sure I don't really care because neither work the way I want a monitor to work. So… when I sat down to create a game plan for cleaning up CloudWatch or setting up Prometheus in a new environment I'm standing up, I chose another option. 

Instead, I built Greybeard. An opinionated monitoring engine built on Unix principles of doing one thing and doing it well. Greybeard is not a complete monitoring *system*, it's just the engine at the core of a system. Greybeard does three things: it runs checks, it monitors the result of the checks, and it runs actions based on those results. It stores (some) state and historical data in a PostgreSQL database, and reads its config from a plain text YAML file. Everything else that's not one of those things, is out of scope for Greybeard. 

That PostgreSQL database is required, Greybeard won't run without it. The `GREYBEARD_DATABASE_URL` environment variable must contain the PostgreSQL connection string. After loading its configuration, Greybeard uses that value to connect to the database. 

Greybeard will check three places for a config file named `greybeard.yml`. 

- A `./config/` directory in the directory that Greybeard is started from.
- A hidden `~/.greybeard/` directory in the user's home. 
- Or for production deployment, in `/etc/greybeard/`. 

You can also explicitly pass a path to `greybeard.yml` with the `-c`, or `--config` flags. 

```
greybeard -c /path/to/greybeard.yml
```

The file should look something like this:


## Configuration

Durations use Go duration syntax such as `30s`, `5m`, or `1h`.

```yaml
startup_spread: 1m

defaults:
  interval: 5m
  timeout: 30s
  attempts: 3
  critical_reminder_interval: 10m

actions:
  - name: notify
    command: /usr/local/greybeard/actions/notify
    timeout: 30s

imports:
  - checks.d/

checks:
  - name: check-disk
    target: web01
    command: /usr/local/greybeard/checks/check_disk
    args:
      - -w
      - 10%
      - -c
      - 5%
      - -p
      - /
    actions:
      critical:
        - notify
```

### Top-level settings

`startup_spread`
: Required. Spreads initial check execution across the specified duration to avoid running every check simultaneously. Use `0` to disable spreading.

`defaults`
: Default values inherited by checks unless overridden.

`imports`
: Additional YAML files or directories containing checks. Relative paths are resolved relative to the main configuration file. Imported files contain `checks:` only. So, no importing from imports. 

`actions`
: Defines named action plugins that checks may reference.

`checks`
: Defines the checks Greybeard executes.

### Default settings

`interval`
: How often a check runs.

`timeout`
: Maximum time a check may run. Must be shorter than its interval.

`attempts`
: Number of consecutive observations required to confirm a state change. 

`critical_reminder_interval`
: How often actions assigned to `critical` are repeated while a check remains CRITICAL. The built-in default is `10m`.

### Checks

Each check supports:

`name`
: Unique name for the check. Check names must be unique across all Greybeard instances sharing the same database.

`target`
: Descriptive name of the system or resource being checked, such as `web01`, `database`, or `example.com`.

`command`
: Executable check plugin.

`args`
: Optional list of command-line arguments. Each argument should be a separate YAML list item.

`interval`, `timeout`, `attempts`, `critical_reminder_interval`
: Optional per-check overrides of the configured defaults.

`actions`
: Named actions to run when a state is confirmed. Actions may be assigned independently to `ok`, `warning`, `critical`, and `unknown`.

```yaml
actions:
  ok:
    - recovered
  warning:
    - notify
  critical:
    - notify
    - page
  unknown:
    - notify
```

Greybeard is designed to work with Nagios-style plugins that use the standard exit codes and plugin output format.

Actions are triggered only by confirmed state transitions. CRITICAL actions are also used for CRITICAL reminders.

### Actions

Actions are defined globally and referenced by name from checks.

```yaml
actions:
  - name: notify
    command: /usr/local/greybeard/actions/notify
    args:
      - --channel
      - operations
    timeout: 30s
```

Each action requires a unique `name`, executable `command`, and `timeout`. `args` is optional. Actions must accept JSON data on `stdin`. 

## Check State

I mentioned previously that Greybeard persists *some* state in the database, but not all. Greybeard does record monitoring state of checks, so if a check returns `CRITICAL` the number of times configured in `attempts`, Greybeard will store that state in the database. 

However, Greybeard does not persist candidate state or reminder schedules. For example, if `attempts = 3` and an OK check returns `CRITICAL` twice, restarting Greybeard before the third observation discards that candidate progress. The check must produce three consecutive `CRITICAL` observations again before the transition is confirmed.

> Observe, then act.

Likewise, if a system was in `CRITICAL` state, and had a critical reminder scheduled to fire, before Greybeard was stopped, Greybeard will not try to "catch up" on missed action executions. Current state will be verified, and new reminders will be scheduled as required. 

On any confirmed transition among `OK`, `WARNING`, `CRITICAL`, and `UNKNOWN`, Greybeard can execute actions assigned to the new state. So, when a check recovers, the sysadmin can know about it. 

A current CRITICAL incident may be acknowledged through PostgreSQL. Acknowledgement suppresses CRITICAL reminders only; checks continue to run and confirmed state transitions still execute their configured actions. The acknowledgement is cleared when the check enters a new confirmed state. 

## What Greybeard is Not

Greybeard will not run on Windows.

Greybeard does not provide a GUI or network API. Greybeard does not provide its own daemonization code, it runs in the foreground and relies on operating system tooling for proper daemon handling. 

Greybeard does not include escalation logic, notification-provider integration, remediation policies, or operator tooling. Each of those jobs, if needed, can live in the checks and actions. It's Greybeard's job to let the action know that something happened. 

Finally, Greybeard is not intended for use in major installations. Greybeard has no high-availability coordination, leader election, or distributed scheduling.  (Although, it should be fairly safe to deploy more than one Greybeard, if the configs are different, and if all the `name` variables for the checks are unique.) If you need the things that Greybeard doesn't offer, maybe Grafana, Prometheus, or any number of other enterprise monitoring tools will be right for you. 

That being said, Greybeard is small, efficient, and flexible. Who knows what it might be good at?

## Future Development

In fact, it's my hope that the design of Greybeard is flexible enough to accommodate a wide range of uses. Eventually I'd like to have separate projects offering the things that Greybeard itself doesn't do.

Eventually I'll setup install packages for the major Linux distributions, and an easy Homebrew recipe for macOS. 

I'd also like to support different databases, most interestingly maybe something like S3 Tables or a distributed SQLite setup. 

I'm going to be using this in my environment, and as I notice things that need fixing or updates that would be beneficial, they'll make their way into the main Greybeard source.

This is going to be fun.

