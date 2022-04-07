package exporter

import (
	"aws-firehose-exporter/metrics"
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

type Cloudwatch struct {
	metricsMap            map[string]*prometheus.GaugeVec
	cloudwatchMetrics     *metrics.CloudwatchMetrics
	registry              *prometheus.Registry
	prefix                string
	refreshMetricsTimeout time.Duration
	lastRefresh           time.Time
}

func NewCloudwatch(prefix string, cloudwatchMetrics *metrics.CloudwatchMetrics, refreshMetricsTimeout time.Duration, registry *prometheus.Registry) *Cloudwatch {
	return &Cloudwatch{
		cloudwatchMetrics:     cloudwatchMetrics,
		metricsMap:            make(map[string]*prometheus.GaugeVec),
		registry:              registry,
		prefix:                prefix,
		refreshMetricsTimeout: refreshMetricsTimeout,
		lastRefresh:           time.Now(),
	}
}

func (c *Cloudwatch) Init() error {
	for _, m := range c.metricsMap {
		c.registry.Unregister(m)
	}

	ctx := context.Background()
	cwMetrics, err := c.cloudwatchMetrics.ListAvailableMetrics(ctx)
	if err != nil {
		return err
	}

	logrus.Infof("discovered %d metrics", len(cwMetrics))

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
		Name: fmt.Sprintf("%s%s", c.prefix, k),
		Help: fmt.Sprintf("%s cloudwatch average value", k),
	}, []string{"resource"})

	c.registry.MustRegister(gauge)
	return gauge
}

func (c *Cloudwatch) Refresh(ctx context.Context) error {
	// force metrics list refresh to avoid stale data
	if time.Since(c.lastRefresh) > c.refreshMetricsTimeout {
		if err := c.Init(); err != nil {
			return err
		}
		c.lastRefresh = time.Now()
	}

	metricsList, err := c.cloudwatchMetrics.Metrics(ctx)
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
