# Test Fixtures

This directory contains baseline outputs for snapshot testing.

## Deterministic Snapshot Targets

Use the Make targets below to generate the same snapshots every time,
independent of your current `kubectl` context.

Default snapshot sources:

- Mock cluster (`--mock`) in namespace `petshop`

```bash
# JSON snapshots
make snapshot-json

# Tree snapshots
make snapshot-tree

# Both JSON + tree snapshots
make snapshot
```

Default output files:

- `testdata/fixtures/mock.json`
- `testdata/fixtures/mock.txt`

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
