# gcloud-stackdriver-log-usage

A tool for analyzing log ingestion for Google Cloud projects to ensure they don't go over the 50G
limit imposed by the [new Stackdriver pricing model](https://cloud.google.com/stackdriver/pricing_v2).

## Usage

```bash
$ go get github.com/mrparkers/gcloud-stackdriver-log-usage
$ gcloud auth application-default login
$ gcloud-stackdriver-log-usage
```
