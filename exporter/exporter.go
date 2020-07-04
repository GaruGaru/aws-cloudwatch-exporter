package exporter

import (
	"aws-firehose-exporter/metrics"
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"regexp"
	"strings"
)

type Cloudwatch struct {
	metricsMap      map[string]*prometheus.GaugeVec
	fireHoseMetrics *metrics.CloudwatchMetrics
	registry        *prometheus.Registry
}

func NewCloudwatch(firehoseMetrics *metrics.CloudwatchMetrics, registry *prometheus.Registry) *Cloudwatch {
	return &Cloudwatch{
		fireHoseMetrics: firehoseMetrics,
		metricsMap:      make(map[string]*prometheus.GaugeVec),
		registry:        registry,
	}
}

func (c *Cloudwatch) Init() error {
	ctx := context.Background()
	cwMetrics, err := c.fireHoseMetrics.ListAvailableMetrics(ctx)
	if err != nil {
		return err
	}

	names := make(map[string]bool)
	for _, m := range cwMetrics {
		names[m.Name] = true
	}

	for k := range names {
		k = normalize(k)
		c.metricsMap[k] = c.createGaugeForMetric(k)

	}

	return nil
}

func (c *Cloudwatch) createGaugeForMetric(k string) *prometheus.GaugeVec {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: k,
		Help: fmt.Sprintf("%s cloudwatch average value", k),
	}, []string{"resource"})

	c.registry.MustRegister(gauge)
	return gauge
}

func (c *Cloudwatch) Refresh(ctx context.Context) error {
	metricsList, err := c.fireHoseMetrics.Metrics(ctx)
	if err != nil {
		return err
	}

	for _, m := range metricsList {
		key := normalize(m.Metric)
		gauge, found := c.metricsMap[key]
		if !found {
			gauge = c.createGaugeForMetric(key)
			c.metricsMap[key] = gauge
		}

		gauge.WithLabelValues(m.Resource).Set(m.Value)
	}

	return nil
}

var removeSpecialChar = regexp.MustCompile(`[^A-Za-z0-9]`)
var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func normalize(str string) string {
	str = removeSpecialChar.ReplaceAllString(str, "")
	str = matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	str = matchAllCap.ReplaceAllString(str, "${1}_${2}")
	return strings.ToLower(str)
}
