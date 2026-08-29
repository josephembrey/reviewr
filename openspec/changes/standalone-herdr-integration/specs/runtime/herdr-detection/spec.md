## Purpose

Define how the standalone reviewr process recognizes an optional Herdr host without depending on
plugin packaging, plugin lifecycle hooks, or pane-management side effects.

## ADDED Requirements

### Requirement: Herdr detection is automatic and environment-based
On startup, reviewr SHALL treat `HERDR_ENV=1` as the authoritative signal that it is running inside
Herdr. It SHALL capture the Herdr workspace, tab, pane, socket, and binary-path values that are
present without invoking the Herdr CLI solely to discover its host.

#### Scenario: Process starts inside Herdr
- **WHEN** reviewr starts with `HERDR_ENV=1` and Herdr context variables are present
- **THEN** the application starts in Herdr-hosted mode with that context available to runtime services

#### Scenario: Process starts outside Herdr
- **WHEN** reviewr starts without `HERDR_ENV=1`
- **THEN** the application starts in standalone mode without requiring a Herdr executable or socket

#### Scenario: Herdr context is partial
- **WHEN** `HERDR_ENV=1` is present but one or more optional Herdr context values are absent
- **THEN** reviewr remains in Herdr-hosted mode and marks only the unavailable host capabilities as unavailable

### Requirement: Host detection is centralized and immutable
reviewr SHALL read host-identifying environment values once during executable startup and SHALL
provide the resulting immutable context to application services. Feature packages SHALL NOT read
Herdr-identifying process environment independently.

#### Scenario: Application initializes its services
- **WHEN** the executable constructs the application
- **THEN** every service receives one consistent host context derived from the startup environment

### Requirement: Standalone execution is the only entry point
reviewr SHALL run as a normal standalone binary and SHALL NOT require a plugin manifest, plugin
installation root, plugin action, or plugin event payload. Herdr integration SHALL activate after the
binary is launched rather than by controlling its installation or pane lifecycle.

#### Scenario: User launches reviewr from a Herdr pane
- **WHEN** a user, layout, or command starts the standalone binary inside a Herdr-managed pane
- **THEN** reviewr detects the host and does not open, close, toggle, or auto-open any additional pane

#### Scenario: User launches reviewr from an ordinary terminal
- **WHEN** the standalone binary is launched outside Herdr
- **THEN** primary repository browsing remains available with no plugin setup

### Requirement: Detection has no external side effects
Host detection SHALL NOT mutate Git state, send agent input, alter Herdr layout, or write application
state. Host capabilities MUST remain separate from detection and preserve the **No writes** and
**Comments survive** invariants.

#### Scenario: Startup detects Herdr
- **WHEN** reviewr recognizes a Herdr environment during startup
- **THEN** detection itself completes without Git writes, Herdr CLI calls, pane changes, or agent messages

### Requirement: Hosted pane labeling is ownership-safe
When hosted context includes a Herdr binary path and pane identity, reviewr SHALL asynchronously label
an otherwise-unlabeled current pane `reviewr`. It SHALL NOT replace a pre-existing label. On normal
exit, it SHALL clear the label only when this process set it and the label remains `reviewr`.

#### Scenario: Hosted pane has no label
- **WHEN** reviewr starts with pane-label capability and the current pane is unlabeled
- **THEN** reviewr labels that pane `reviewr` without delaying the first application frame

#### Scenario: Hosted pane has a custom label
- **WHEN** reviewr starts and the current pane already has a label
- **THEN** reviewr leaves that label unchanged and does not claim ownership of it

#### Scenario: Owned label remains at normal exit
- **WHEN** reviewr set the pane label and it still equals `reviewr` during normal shutdown
- **THEN** reviewr clears the label

#### Scenario: Owned label was replaced externally
- **WHEN** reviewr set the pane label but another actor subsequently changes it
- **THEN** reviewr leaves the replacement label unchanged during shutdown

#### Scenario: Pane-label capability is unavailable
- **WHEN** reviewr runs standalone or hosted context lacks either the Herdr binary path or pane identity
- **THEN** reviewr performs no pane-label command and otherwise behaves normally
