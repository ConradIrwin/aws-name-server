package main

import (
	"fmt"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
	"log"
	"net"
	"sync"
	"time"
)

// The length of time to cache the results of ec2-describe-instances.
// This value is exposed as the TTL of the DNS record (down to a minimum
// of 10 seconds).
const TTL = 5 * time.Minute

// LookupTag represents the type of tag we're caching by.
type LookupTag uint8

const (
	// LOOKUP_NAME for when tag:Name=<value>
	LOOKUP_NAME LookupTag = iota
	// LOOKUP_ROLE for when tag:Role=<value>
	LOOKUP_ROLE
)

// Key is used to cache results in O(1) lookup structures.
type Key struct {
	LookupTag
	string
}

// Record represents the DNS record for one EC2 instance.
type Record struct {
	CName      string
	PublicIP   net.IP
	PrivateIP  net.IP
	ValidUntil time.Time
}

// EC2Cache maintains a local cache of ec2-describe-instances data.
// It refreshes every TTL.
type EC2Cache struct {
	region    aws.Region
	accessKey string
	secretKey string
	records   map[Key][]*Record
	mutex     sync.RWMutex
}

// NewEC2Cache creates a new EC2Cache that uses the provided
// EC2 client to lookup instances. It starts a goroutine that
// keeps the cache up-to-date.
func NewEC2Cache(regionName, accessKey, secretKey string) (*EC2Cache, error) {

	region, ok := aws.Regions[regionName]
	if !ok {
		return nil, fmt.Errorf("unknown AWS region: %s", regionName)
	}

	cache := &EC2Cache{
		region:    region,
		accessKey: accessKey,
		secretKey: secretKey,
		records:   make(map[Key][]*Record),
	}

	if err := cache.refresh(); err != nil {
		return nil, err
	}

	go func() {
		for _ = range time.Tick(1 * time.Minute) {
			err := cache.refresh()
			if err != nil {
				log.Println("ERROR: " + err.Error())
			}
		}
	}()

	return cache, nil
}

// setRecords updates the cache with a new set of Records
func (cache *EC2Cache) setRecords(records map[Key][]*Record) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.records = records
}

func (cache *EC2Cache) Instances() (*ec2.InstancesResp, error) {
	auth, err := aws.GetAuth(cache.accessKey, cache.secretKey)
	if err != nil {
		return nil, err
	}

	return ec2.New(auth, cache.region).Instances(nil, nil)
}

func (cache *EC2Cache) refresh() error {
	result, err := cache.Instances()
	validUntil := time.Now().Add(TTL)

	if err != nil {
		return err
	}

	records := make(map[Key][]*Record)
	count := 0

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			count++
			record := Record{}
			if instance.PublicIpAddress != "" {
				record.PublicIP = net.ParseIP(instance.PublicIpAddress)
			}
			if instance.PrivateIpAddress != "" {
				record.PrivateIP = net.ParseIP(instance.PrivateIpAddress)
			}
			if instance.DNSName != "" {
				record.CName = instance.DNSName + "."
			}
			record.ValidUntil = validUntil
			for _, tag := range instance.Tags {
				if tag.Key == "Name" {
					records[Key{LOOKUP_NAME, tag.Value}] = append(records[Key{LOOKUP_NAME, tag.Value}], &record)
				}
				if tag.Key == "Role" {
					records[Key{LOOKUP_ROLE, tag.Value}] = append(records[Key{LOOKUP_ROLE, tag.Value}], &record)
				}
			}
		}
	}
	cache.setRecords(records)
	return nil
}

// Lookup a node in the Cache either by Name or Role.
func (cache *EC2Cache) Lookup(tag LookupTag, value string) []*Record {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	return cache.records[Key{tag, value}]
}

func (cache *EC2Cache) Size() int {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	return len(cache.records)
}

func (record *Record) TTL(now time.Time) time.Duration {
	if now.After(record.ValidUntil) {
		return 10 * time.Second
	}
	return record.ValidUntil.Sub(now)
}
