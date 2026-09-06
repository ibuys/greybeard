# Greybeard Configs

> "Observe, Then Decide"

Greybeard looks for one configuration file one of three places, in this order:

- `./config/greybeard.yml`
- `~/.greybeard/greybeard.yml`
- `/etc/greybeard/greybeard.yml`

Greybeard stops once it finds a config file, and the containing directory becomes the config home. The main `greybeard.yml` file can contain an `imports:` section that will point at other files or directories *relative to the config home*. For example, this `config/greybeard.yml` file:

```
imports:
    - broken.yml
    - ok.yml
    - metrics
```

Will load `config/broken.yml`, `config/ok.yml`, and all `.yml` or `.yaml` files found in `config/metrics/`. However, import statements in other files are ignored. Also, directory statements are non-recursive, Greybeard will only go one layer deep.

The layout of the config files is intentionally kept simple, but flexible:

```
checks:
  - name: check-metrics-quoted
    command: ./test_checks/check-metrics-quoted.sh
    args: []
    timeout: 45s
    interval: 1m

  - name: check-metrics
    command: ./test_checks/check-metrics.sh
    args: []
    timeout: 45s
    interval: 1m
```

This config file names each check, sets the command for Greybeard to run, provides any arguments to the command, a timeout that will kill the check and provide an error back to Greybeard, and how often Greybeard should run the check.

There is also an optional supported `args` setting, for commands that require arguments. 