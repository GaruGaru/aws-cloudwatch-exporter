package metrics

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/patrickmn/go-cache"
	"regexp"
	"strings"
	"time"
)

type Metric struct {
	Resource string
	Metric   string
	Value    float64
}

type CloudwatchMetrics struct {
	cloudWatch     *cloudwatch.CloudWatch
	namespace      string
	allowedMetrics []string
	cache          *cache.Cache
}

func New(cw *cloudwatch.CloudWatch, namespace string, filteredMetrics []string) *CloudwatchMetrics {
	return &CloudwatchMetrics{
		cloudWatch:     cw,
		namespace:      namespace,
		allowedMetrics: filteredMetrics,
		cache:          cache.New(60*time.Minute, 120*time.Minute),
	}
}

func (f *CloudwatchMetrics) Metrics(ctx context.Context) ([]Metric, error) {
	am, err := f.ListAvailableMetrics(ctx)
	if err != nil {
		return nil, err
	}

	return f.queryMetrics(ctx, am)
}

var removeSpecialChar = regexp.MustCompile(`[^A-Za-z0-9]`)

func createMetricsDataRequest(metrics []DimensionWithMetric, period int64, stat string) []*cloudwatch.MetricDataQuery {
	requests := make([]*cloudwatch.MetricDataQuery, len(metrics))
	for i, metric := range metrics {
		requests[i] = &cloudwatch.MetricDataQuery{
			Id: aws.String(
				fmt.Sprintf("id_%d", i),
			),
			Label: aws.String(fmt.Sprintf("%s::%s", metric.DimensionValue, metric.Name)),
			MetricStat: &cloudwatch.MetricStat{
				Metric: &cloudwatch.Metric{
					Namespace: aws.String(metric.Namespace),
					Dimensions: []*cloudwatch.Dimension{
						{
							Name:  aws.String(metric.DimensionName),
							Value: aws.String(metric.DimensionValue),
						},
					},
					MetricName: aws.String(metric.Name),
				},
				Period: aws.Int64(period), //100 * 60
				Stat:   aws.String(stat),  //"Average"
			},
			ReturnData: aws.Bool(true),
		}
	}

	return requests
}

type DimensionWithMetric struct {
	DimensionName  string
	DimensionValue string
	Name           string
	Namespace      string
}

func (f *CloudwatchMetrics) queryMetrics(ctx context.Context, am []DimensionWithMetric) ([]Metric, error) {
	metricsRequest := createMetricsDataRequest(am, 600, "Average")

	metricsRequestBatch := chunk(metricsRequest, 500)

	metrics := make([]Metric, 0)

	for _, batch := range metricsRequestBatch {
		err := f.cloudWatch.GetMetricDataPagesWithContext(ctx, &cloudwatch.GetMetricDataInput{
			StartTime:         aws.Time(time.Now().Add(-10 * time.Minute)),
			EndTime:           aws.Time(time.Now()),
			MetricDataQueries: batch,
		}, func(output *cloudwatch.GetMetricDataOutput, b bool) bool {
			for _, data := range output.MetricDataResults {
				split := strings.Split(*data.Label, "::")
				resource := split[0]
				metric := split[1]

				if len(data.Values) == 0 {
					continue
				}

				metrics = append(metrics, Metric{
					Resource: resource,
					Metric:   metric,
					Value:    *data.Values[0],
				})
			}

			return true
		})

		if err != nil {
			return nil, err
		}

	}
	return metrics, nil
}

func (f *CloudwatchMetrics) ListAvailableMetrics(ctx context.Context) ([]DimensionWithMetric, error) {
	const cacheKey = "ListAvailableMetrics"
	cached, present := f.cache.Get(cacheKey)
	if present {
		return cached.([]DimensionWithMetric), nil
	}

	dwm := make([]DimensionWithMetric, 0)

	err := f.cloudWatch.ListMetricsPagesWithContext(ctx, &cloudwatch.ListMetricsInput{
		Namespace: aws.String(f.namespace),
	}, func(output *cloudwatch.ListMetricsOutput, b bool) bool {
		for _, metric := range output.Metrics {
			for _, dim := range metric.Dimensions {
				if !f.isMetricAllowed(*metric.MetricName) {
					continue
				}

				dwm = append(dwm, DimensionWithMetric{
					DimensionName:  *dim.Name,
					DimensionValue: *dim.Value,
					Name:           *metric.MetricName,
					Namespace:      f.namespace,
				})
			}
		}
		return true
	})

	f.cache.Set(cacheKey, dwm, 1*time.Hour)
	return dwm, err

}

func (f *CloudwatchMetrics) isMetricAllowed(name string) bool {
	if len(f.allowedMetrics) == 0 {
		return true
	}

	for _, allowed := range f.allowedMetrics {
		if allowed == name {
			return true
		}
	}

	return false
}

func chunk(rows []*cloudwatch.MetricDataQuery, chunkSize int) [][]*cloudwatch.MetricDataQuery {
	var chunk []*cloudwatch.MetricDataQuery
	chunks := make([][]*cloudwatch.MetricDataQuery, 0, len(rows)/chunkSize+1)

	for len(rows) >= chunkSize {
		chunk, rows = rows[:chunkSize], rows[chunkSize:]
		chunks = append(chunks, chunk)
	}

	if len(rows) > 0 {
		chunks = append(chunks, rows[:len(rows)])
	}

	return chunks
}
