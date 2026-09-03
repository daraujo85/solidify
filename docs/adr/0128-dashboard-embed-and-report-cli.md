# 0098. Dashboard Embed and Report CLI

Date: 2026-08-30

## Status

Accepted

## Context

We need a way to serve the dashboard UI without requiring users to host a separate web server. We also need to support generating PDF reports for compliance and archiving, while maintaining consistency with the interactive dashboard. Previously, we considered exporting static HTML, but this adds complexity and duplicates functionality.

## Decision

1.  **Dashboard Embedding**: We will use `//go:embed` to embed the dashboard UI assets (HTML, CSS, JS) directly into the Solidify binary. This allows users to run `solidify dashboard` and instantly view the interactive dashboard without any external dependencies.
2.  **PDF Report Generation**: We will use an ephemeral server to render the embedded dashboard UI and then use a headless browser (or similar tool) to capture a PDF. This ensures the PDF report perfectly matches the interactive dashboard, eliminating the need to maintain a separate PDF generation template.
3.  **Drop HTML Export**: We will drop support for exporting static HTML reports. The `solidify dashboard` command provides a superior interactive experience, and the `solidify report --format pdf` command satisfies the need for static reports.

## Consequences

*   **Pros**:
    *   Simplified distribution: The dashboard is built directly into the binary.
    *   Consistent rendering: The PDF report matches the dashboard exactly.
    *   Reduced maintenance: No need to maintain separate HTML export templates.
*   **Cons**:
    *   Slightly larger binary size due to embedded assets.
    *   PDF generation requires an ephemeral server and a headless browser, which may increase resource usage during report generation.
