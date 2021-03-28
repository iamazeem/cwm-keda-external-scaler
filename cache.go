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
	c.cache = append(c.cache, metricData{
		timestamp:   time.Now(),
		metricValue: metricValue,
	})
}

func (c *metricCache) purge(scalePeriodSeconds int64) {
	log.Printf("purging metric values [%v: %v]", keyScalePeriodSeconds, scalePeriodSeconds)

	if c.isEmpty() {
		log.Println("purge failed! cache is empty!")
		return
	}

	// remove values with timestamps with difference older than scalePeriodSeconds
	// e.g. if scalePeriodSeconds = 600, all the values with difference >= 600 will be removed
	oldCacheSize := c.getSize()
	now := time.Now()
	for i, d := range c.cache {
		seconds := int64(now.Sub(d.timestamp).Seconds())
		if seconds < scalePeriodSeconds {
			c.cache = c.cache[i:]

			newCacheSize := c.getSize()
			noOfValuesPurged := oldCacheSize - newCacheSize
			log.Printf("purged %v values. cache size: {old: %v, new: %v}", noOfValuesPurged, oldCacheSize, newCacheSize)
			break
		}
	}
}

func (c *metricCache) getOldestMetricData() (metricData, error) {
	if c.isEmpty() {
		return metricData{}, status.Errorf(codes.NotFound, "cache is empty")
	}

	return c.cache[0], nil
}
