package main

import (
	"aws-firehose-exporter/exporter"
	"aws-firehose-exporter/metrics"
	"context"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", "0.0.0.0", "web server bind address")
	port := flag.Int("port", 9999, "web server port")
	metricsPath := flag.String("path", "/metrics", "exporter metrics path")
	refreshRate := flag.Int("refresh", 2*60, "refresh delay in seconds")
	allowedMetricsFlag :=	flag.String("allow-metric", "", "list of metric to select (eg: messages,queue-size...)")
	cloudwatchNamespace := flag.String("cloudwatch-namespace", "", "cloudwatch metric namespaces (eg AWS/Firehose)")

	flag.Parse()

	if cloudwatchNamespace == nil || *cloudwatchNamespace == "" {
		panic("no cloudwatch-namespace set")
	}

	allowedMetrics := strings.Split(*allowedMetricsFlag, ",")

	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	registry := prometheus.NewRegistry()

	sess := session.Must(session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable}))

	svc := cloudwatch.New(sess)
	cwExporter := exporter.NewCloudwatch(metrics.New(svc, *cloudwatchNamespace, allowedMetrics), registry)

	if err := cwExporter.Init(); err != nil {
		panic(err)
	}

	ticker := time.NewTicker(time.Duration(*refreshRate) * time.Second)

	ctx := context.Background()
	err := cwExporter.Refresh(ctx)

	if err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := cwExporter.Refresh(ctx); err != nil {
					log.Error(err)
				}
			}
		}
	}()

	http.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	bind := fmt.Sprintf("%s:%d", *addr, *port)

	log.Infof("started metrics server on %s", bind)

	if err := http.ListenAndServe(bind, nil); err != nil {
		panic(err)
	}
}
