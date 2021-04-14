package main

import (
	"log"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	cache metricCache
)

type metricData struct {
	timestamp   time.Time
	metricValue int64
}

type metricCache struct {
	cache []metricData
}

func (c *metricCache) getSize() int {
	return len(c.cache)
}

func (c *metricCache) isEmpty() bool {
	return c.getSize() == 0
}

func (c *metricCache) append(metricValue int64) {
	// append value if cache is empty or the metric value is greater than the last one
	if c.isEmpty() || metricValue > c.cache[c.getSize()-1].metricValue {
		c.cache = append(c.cache, metricData{
			timestamp:   time.Now().UTC(),
			metricValue: metricValue,
		})
	}
}

func (c *metricCache) getPurgeIndex(scalePeriodSeconds int64) int64 {
	var index int64 = 0

	now := time.Now().UTC()
	for _, d := range c.cache {
		seconds := now.Sub(d.timestamp).Seconds()
		if seconds > float64(scalePeriodSeconds) {
			index++
		}
	}

	return index
}

func (c *metricCache) purge(scalePeriodSeconds int64) {
	log.Printf("purging metric values [%v: %v]", keyScalePeriodSeconds, scalePeriodSeconds)

	if c.isEmpty() {
		log.Println("purge failed! cache is empty!")
		return
	}

	// remove values with timestamps with difference older than scalePeriodSeconds
	// e.g. if scalePeriodSeconds = 600, all the values with difference >= 600 will be removed
	purgeIndex := c.getPurgeIndex(scalePeriodSeconds)
	log.Printf("number of values to purge: %v", purgeIndex)

	if purgeIndex > 0 {
		oldCacheSize := c.getSize()
		c.cache = c.cache[purgeIndex:]
		newCacheSize := c.getSize()
		noOfValuesPurged := oldCacheSize - newCacheSize
		log.Printf("purged %v value(s). cache size: {old: %v, new: %v}", noOfValuesPurged, oldCacheSize, newCacheSize)
	}
}

func (c *metricCache) getOldestMetricData() (metricData, error) {
	if c.isEmpty() {
		return metricData{}, status.Errorf(codes.NotFound, "cache is empty")
	}

	return c.cache[0], nil
}
