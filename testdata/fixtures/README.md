# Test Fixtures

This directory contains snapshot fixtures used by tests and manual regression checks.

## Snapshot Commands

Use the Make targets below:

```bash
# Deterministic mock fixtures (default)
make snapshot

# Real-cluster fixtures (requires valid context)
make snapshot-real
```

## Fixture Files

- `mock.json`: mock graph response snapshot (`--json --mock`)
- `mock.txt`: mock tree text snapshot (`--mock`)
- `real.json`: real-cluster graph response snapshot (`--json`)
- `real.txt`: real-cluster tree text snapshot

The snapshot tests currently load `mock.json` as the stable input fixture.
