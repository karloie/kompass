# Test Fixtures

This directory contains baseline outputs for snapshot testing.

## Deterministic Snapshot Targets

Use the Make targets below to generate the same snapshots every time,
independent of your current `kubectl` context.

Default snapshot sources:

- Mock cluster (`--mock`) in namespace `petshop`
- Real cluster context `tool-test-01` in namespace `applikasjonsplattform`

```bash
# JSON snapshots
make snapshot-json

# Tree snapshots
make snapshot-tree

# Both JSON + tree snapshots
make snapshot
```

Default output files:

- `testdata/fixtures/kompass_snapshot_mock.json`
- `testdata/fixtures/kompass_snapshot_tool_app.json`
- `testdata/fixtures/kompass_snapshot_mock.txt`
- `testdata/fixtures/kompass_snapshot_tool_app.txt`

You can override contexts, namespaces, or output directory:

```bash
make snapshot-json SNAPSHOT_DIR=/tmp SNAPSHOT_MOCK_NAMESPACE=petshop SNAPSHOT_TOOL_CONTEXT=tool-test-01 SNAPSHOT_TOOL_NAMESPACE=applikasjonsplattform
```

## Fixtures

### mock_output_baseline.txt
Baseline output from `make mock` command, showing the complete tree visualization of the mock Kubernetes cluster. This snapshot is used to detect regressions in tree rendering over time.

To update the baseline after intentional changes:
```bash
make mock > testdata/fixtures/mock_output_baseline.txt
```

To compare current output with baseline:
```bash
make mock > /tmp/current.txt
diff testdata/fixtures/mock_output_baseline.txt /tmp/current.txt
```
