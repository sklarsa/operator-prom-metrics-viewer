# Operator Metrics Viewer

This is a little CLI tool that scrapes and displays Prometheus metrics from a running operator.  I use this when profiling the performance of operators while developing and testing them.

## Screenshot
<img width="1496" alt="Screen Shot 2023-07-11 at 2 12 02 PM" src="https://github.com/sklarsa/operator-prom-metrics-viewer/assets/1929541/d2970e9a-8190-450e-ace7-647ad5957ac1">

## Running

This project is in its very early stages, so I haven't set up a build process. To run the tool, simply run `go run ./ $METRICS_HOST`, setting `$METRICS_HOST` to the default metric address (and port) that your controller manager exposes.
