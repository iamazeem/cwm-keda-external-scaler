package main

import (
	"time"

	log "github.com/sirupsen/logrus"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	cache metricCache
)

type metricData struct {
	timestamp time.Time
	metric    metric
}

type metricCache struct {
	cache map[string][]metricData // map: deploymentid => metricData
}

func (c *metricCache) initializeIfNil() {
	if c.cache == nil {
		c.cache = make(map[string][]metricData)
		log.Debug("cache initialized")
	}
}

func (c *metricCache) getSize(deploymentid string) int {
	return len(c.cache[deploymentid])
}

func (c *metricCache) isEmpty(deploymentid string) bool {
	return c.getSize(deploymentid) == 0
}

func (c *metricCache) append(deploymentid string, metric metric, scalePeriodSeconds int64) {
	c.initializeIfNil()

	log.Debugf("[deploymentid: %v] appending metric {name: %v, value: %v}", deploymentid, metric.name, metric.value)

	c.cache[deploymentid] = append(c.cache[deploymentid], metricData{
		timestamp: time.Now().UTC(),
		metric:    metric,
	})

	log.Debugf("[deploymentid: %v] appended metric {name: %v, value: %v}", deploymentid, metric.name, metric.value)

	c.purge(deploymentid, scalePeriodSeconds)
}

func (c *metricCache) getPurgeIndex(deploymentid string, scalePeriodSeconds int64) int64 {
	var index int64 = 0

	now := time.Now().UTC()
	for _, d := range c.cache[deploymentid] {
		seconds := now.Sub(d.timestamp).Seconds()
		if seconds > float64(scalePeriodSeconds) {
			index++
		}
	}

	log.Debugf("[deploymentid: %v] number of values to purge: %v", deploymentid, index)

	return index
}

func (c *metricCache) purge(deploymentid string, scalePeriodSeconds int64) {
	log.Debugf("[deploymentid: %v] purging metric values [%v = %v]", deploymentid, keyScalePeriodSeconds, scalePeriodSeconds)

	if c.isEmpty(deploymentid) {
		log.Debugf("[deploymentid: %v] cache is already empty, purge not needed", deploymentid)
		return
	}

	// remove values with timestamps with difference older than scalePeriodSeconds
	// e.g. if scalePeriodSeconds = 600, all the values with difference >= 600 will be removed
	purgeIndex := c.getPurgeIndex(deploymentid, scalePeriodSeconds)
	if purgeIndex > 0 {
		oldCacheSize := c.getSize(deploymentid)
		c.cache[deploymentid] = c.cache[deploymentid][purgeIndex:]
		newCacheSize := c.getSize(deploymentid)
		noOfValuesPurged := oldCacheSize - newCacheSize
		log.Infof("[deploymentid: %v] purged %v value(s). cache size: {old: %v, new: %v}", deploymentid, noOfValuesPurged, oldCacheSize, newCacheSize)
	}

	// after purging values, if a cache's list for a certain deploymentid is empty,
	// it's best to purge its slot completely also instead of retaining its memory,
	// for the same deploymentid, the slot will be added again if it reappears later
	if c.isEmpty(deploymentid) {
		delete(c.cache, deploymentid)
		log.Infof("[deploymentid: %v] empty cache slot purged completely", deploymentid)
	}
}

func (c *metricCache) getOldestMetricData(deploymentid string) (metricData, error) {
	if c.isEmpty(deploymentid) {
		return metricData{}, status.Errorf(codes.NotFound, "[deploymentid: %v] cache is empty", deploymentid)
	}

	return c.cache[deploymentid][0], nil
}
