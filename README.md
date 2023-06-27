# Operator Metrics Viewer

This is a little CLI tool that scrapes and displays Prometheus metrics from a running operator.  I use this when profiling the performance of operators while developing and testing them.

## Screenshot

## Running

This project is in its very early stages, so I haven't set up a build process. To run the tool, simply run `go run ./ $METRICS_HOST`, setting `$METRICS_HOST` to the default metric address (and port) that your controller manager exposes.
