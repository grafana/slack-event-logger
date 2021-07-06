# slack-event-logger

Listens for Slack message/reaction events and logs them to the console. This is useful to ingest them into [Loki](https://github.com/grafana/loki) to run analytics on them. Also exposes Prometheus metrics about ingested messages
