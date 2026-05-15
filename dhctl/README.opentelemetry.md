# Working with OpenTelemetry for dhctl

This document explains how to enable, collect and view OpenTelemetry traces produced by dhctl.

## Overview
dhctl can produce traces and metrics for a single run. When tracing is enabled, dhctl writes a local trace file containing all traces and metrics from that run. You can inspect the file locally with the lovie utility or convert it for ingestion into tracing backends such as Grafana Tempo.

Important: Traces may include sensitive data (arguments, resource identifiers, etc.). Treat trace files accordingly.

## Enabling tracing
To enable tracing for a dhctl run, set the environment variable:
- `DHCTL_TRACE=yes`

Example:
- `DHCTL_TRACE=yes dhctl <command>`

dhctl will print information about the created trace file at the very beginning of its output.

## Local trace file
- File name format: `trace-%date.jsonl`
- Location: created alongside dhctl logs (working directory or configured log directory)
- Format: JSON Lines (each line is a JSON object representing spans/metrics/events)
- Content: All traces and metrics produced during that dhctl run

Example log output (informational):
- `Trace file: /tmp/dhctl/trace-20260514151839.jsonl`

## Viewing traces locally with lovie
[lovie](https://github.com/090809/lovie) can render the trace file in a browser for visual inspection.

Basic usage:
- `lovie /path/to/trace-file.jsonl`

This command will open your default web browser and display the run progress and spans.

Converting to Tempo-compatible export:
- `lovie tempo-export /path/to/trace-file.jsonl`
  This converts the dhctl trace format into a trace file suitable for importing into Grafana Tempo (or your DOP). After conversion, follow your backend's import instructions to load the trace.

## Security and privacy
- Traces may include CLI arguments, file paths, resource names and other identifiers. Scrub or redact sensitive traces before sharing with third parties.
- Consider running dhctl with tracing only in trusted environments.

## Known limitations
- Some spans may not be nested correctly in the visualization.
- Some function names or attributes might be incomplete in the current viewer/export path.
- Conversion/import tooling may require backend-specific adjustments.
