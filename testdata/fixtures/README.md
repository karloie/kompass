# Test Fixtures

This directory contains baseline outputs for snapshot testing.

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
