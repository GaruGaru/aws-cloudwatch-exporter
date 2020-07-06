# aws-cloudwatch-exporter
[![Go Report Card](https://goreportcard.com/badge/github.com/GaruGaru/aws-sqs-exporter)](https://goreportcard.com/report/github.com/GaruGaru/aws-sqs-exporter)
[![MicroBadger Size](https://img.shields.io/microbadger/image-size/garugaru/sqs-exporter)](https://cloud.docker.com/u/garugaru/repository/docker/garugaru/sqs-exporter)
 
Prometheus exporter for aws cloudwatch metrics with async refresh

## Running

### Docker
```bash
docker run -it \
 -e AWS_REGION=<region> \
 -e AWS_ACCESS_KEY_ID=<access-key> \
 -e AWS_SECRET_ACCESS_KEY=<secret> \
 -p 9999:9999 \
 garugaru/aws-cloudwatch-exporter -cloudwatch-namespace=AWS/SQS
```
 
