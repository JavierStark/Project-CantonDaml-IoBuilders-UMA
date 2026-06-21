# Simple Topology Tutorial

This tutorial shows a simple Canton setup with three participants named `participant1`, `participant2`, and `participant3`, and a domain named `mydomain`, all running in a single process.

How to run the example is also featured in the [getting started tutorial](https://docs.daml.com/canton/tutorials/getting_started.html#starting-canton).

The example is split across two files:

- `simple-topology.conf` defines the in-memory topology with one domain and three participants.
- `simple-ping.canton` starts the local nodes, connects all participants to `mydomain`, waits for activation, and pings between participants to verify the setup.

The topology uses the following ports:

- `mydomain`: public API `5018`, admin API `5019`
- `participant1`: ledger API `5011`, admin API `5012`
- `participant2`: ledger API `5021`, admin API `5022`
- `participant3`: ledger API `5031`, admin API `5032`

The simple topology example can be invoked using:

```bash
../../canton/canton-open-source-2.10.4/bin/canton -c simple-topology.conf --bootstrap simple-ping.canton
```

When the bootstrap script runs, it starts all configured local instances, bootstraps the domain setup, connects each participant to `mydomain`, waits for all participants to become active, and then performs health pings to confirm that the participants are connected successfully.

## Accessing the Ledger via HTTP (JSON API)

WIP

### Canton REPL Reference Guide

**Basic Console Navigation**
The Canton REPL operates using standard Scala syntax. You can view all available top-level commands, generic node references, and command groups by typing `help`. To cleanly terminate your console session, use the `exit` command.

**Node References and State Verification**
Nodes are accessed directly by their configuration names. You can target collections of nodes using `domains` or `participants`, or interact with specific instances like `participant1` or `mydomain`. To check the operational status of any specific node, append state methods to the node reference, such as executing `participant1.is_initialized` or `participant1.is_running`, which return boolean values.

**Navigating Command Groups**
Command discovery relies on dot notation. To see available actions for a specific node, append the help command, formatted as `participant1 help` or `mydomain help`. This reveals sub-modules like `topology`, `health`, or `keys`. You can chain these modules to explore further, utilizing commands like `participant1.topology.all help` or `mydomain.participants help` to list the specific methods available within those subgroups.

**Domain Participant Management**
Managing and verifying participants on a domain is handled through the domain's participant module. Executing `mydomain.participants.list` retrieves the detailed authorization states and transaction signatures for all participants registered on that domain. To quickly verify if a single participant holds active permissions, use the active method alongside the target node, formatted as `mydomain.participants.active participant1`.
