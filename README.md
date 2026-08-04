# dcna-questions

## Automatic End-to-End Tests Subset

The `e2e/` directory holds the **automated** end-to-end tests — tests that exercise the full HTTP stack against an in-process `httptest.Server` via `go test`.

It is not the only place e2e testing happens: manual e2e checks (e.g. running the server and driving it with `curl` or a browser) are performed outside this directory and are not covered here.

You are still encouraged to cook / orchestrate your own e2e testing flows.

### Running the e2e suite

```sh
go test ./e2e/... -v -timeout 30s
```
