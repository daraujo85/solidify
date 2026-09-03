# Solidify in CI/CD

Solidify is designed to be easily integrated into any CI/CD pipeline. It runs entirely in Docker, requiring no background daemon or complex setup.

## Running in CI

To run Solidify in your CI pipeline, use the provided Docker image. This ensures a consistent environment for building, testing, and generating reports.

```bash
# Run tests and generate reports using the Docker image
docker run --rm -v "$(pwd):/app" -w /app golang:1.27 go test ./...
```

## Pipeline Stages

A typical Solidify CI/CD pipeline consists of the following stages:

1.  **Build**: Compile the application and ensure there are no build errors.
2.  **Test**: Run unit and integration tests.
3.  **Doctor**: Run `solidify doctor` to check the environment and dependencies.
4.  **Dashboard**: Run `solidify dashboard` to generate the interactive HTML dashboard (optional, but recommended for visual review).
5.  **Report**: Run `solidify report --format pdf` to generate a PDF report for compliance or archiving.

## Advantages

*   **No Daemon Required**: Solidify runs as a simple command-line tool, eliminating the need for a background daemon.
*   **Docker-First**: The official Docker image provides a reproducible environment, avoiding "it works on my machine" issues.
*   **Easy Integration**: The simple command-line interface makes it easy to integrate with any CI/CD platform (GitHub Actions, GitLab CI, Jenkins, etc.).
