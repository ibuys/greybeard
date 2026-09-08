# Greybeard

> Observe, then decide.

An old school monitoring system, built for the modern environment.

WIP. 

Just stashing this here as a reminder to rewrite all of this. 

Greybeard does not persist or recover scheduler state. On every startup, it reads configuration, verifies database availability, loads persisted monitoring state, and schedules all configured checks from scratch. Missed check executions are not replayed.

PostgreSQL is required runtime infrastructure for Greybeard. If Greybeard cannot read or write its persistent state, it exits rather than continuing in a degraded mode.

Transitions alert. Persistent bad states remind. Checks determine state; reminder timers determine reminder cadence. Candidates do neither.

When a critical reminder is about to fire, Greybeard first checks the database to verify if the critical check has been acknowledged or not. If it has, Greybeard does not fire off the reminder action. 

Greybeard does not provide a GUI, there is no API outside of the config files. 

Greybeard does not include escalation logic, notification-provider integrations, remediation policies, or operator tooling. Those can live in the checks and actions. It's Greybeard's job to let the actions know when something has happened.
