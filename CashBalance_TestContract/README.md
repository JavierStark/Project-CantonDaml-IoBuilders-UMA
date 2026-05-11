### Build package

Build the DAML project first so the DAR package is generated and ready to use.

```bash
dpm build
```

After the build completes, the package is created at `.daml/dist/cash-balance-test-contract-1.0.0.dar`.

### Run Canton sandbox

There are two ways to start the sandbox, depending on whether you want the DAR loaded automatically or prefer to upload it yourself.

**Option 1: start the sandbox with the DAR already loaded.** This matches the Canton JSON API tutorial.

```bash
dpm sandbox --json-api-port 7575 --dar .daml/dist/cash-balance-test-contract-1.0.0.dar
```

**Option 2: start the sandbox and upload the package manually.** Use this if you want to inspect the package before uploading it.

```bash
dpm sandbox --json-api-port 7575
```

If you use the second option, follow these extra steps:

Get the package ID:

```bash
dpm damlc inspect-dar --json .daml/dist/cash-balance-test-contract-1.0.0.dar | jq '.main_package_id'
```

Upload the package:

```bash
curl --data-binary @.daml/dist/cash-balance-test-contract-1.0.0.dar http://localhost:7575/v2/packages
```

Check which packages are available:

```bash
curl -s http://localhost:7575/v2/packages
curl -s http://localhost:7575/v2/packages | jq '.packageIds | .[] | select(startswith("<first numbers of id>"))'
```

### API references

Use these references if you need more background on the JSON API or its endpoints:

- [Overview](https://docs.digitalasset.com/build/3.4/explanations/json-api/index.html)
- [OpenAPI](https://docs.digitalasset.com/build/3.4/reference/json-api/openapi.html)

### Create parties and contracts

This section walks through the basic flow: create a party, create a contract, inspect active contracts, and then archive the contract.

Create a party. The example below uses Alice:

```bash
curl -d '{"partyIdHint":"Alice"}' http://localhost:7575/v2/parties
```

Confirm the party was created:

```bash
curl http://localhost:7575/v2/parties | jq
```

Create the contract using the prepared payload. Before running it, update `create.json` so the `<party>` values match the parties you created:

```bash
curl localhost:7575/v2/commands/submit-and-wait -H "Content-Type: application/json" -d@create.json | jq
```

Save the offset from the response, because you will need it for the next request.

Check active contracts. Before running this command, update `acs.json` with the correct offset from the previous response:

```bash
curl localhost:7575/v2/state/active-contracts -H "Content-Type: application/json" -d@acs.json | jq
```

Archive the contract and then query again with the new offset to verify the change:

```bash
curl localhost:7575/v2/commands/submit-and-wait -H "Content-Type: application/json" -d@archive.json | jq
```

Check contracts again with the new offset.