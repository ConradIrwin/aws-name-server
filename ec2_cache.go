package main

import (
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
	// tag:Name=<value>
	LOOKUP_NAME LookupTag = iota
	// tag:Role=<value>
	LOOKUP_ROLE
)

// Key is used to cache results in O(1) lookup structures.
type Key struct {
	LookupTag
	string
}

// Record represents the DNS record for one EC2 instance.
type Record struct {
	PublicIP   net.IP
	PrivateIP  net.IP
	ValidUntil time.Time
}

// EC2Cache maintains a local cache of ec2-describe-instances data.
// It refreshes every TTL.
type EC2Cache struct {
	*ec2.EC2
	records    map[Key][]*Record
	InProgress map[Key]time.Time
	mutex      sync.Mutex
}

// NewEC2Cache creates a new EC2Cache that uses the provided
// EC2 client to lookup instances. It starts a goroutine that
// keeps the cache up-to-date.
func NewEC2Cache(cli *ec2.EC2) *EC2Cache {
	cache := &EC2Cache{
		cli,
		make(map[Key][]*Record),
		make(map[Key]time.Time),
		sync.Mutex{},
	}

	go func() {
		for {
			cache.refresh()
			time.Sleep(TTL)
		}
	}()

	return cache
}

// Records contains all the Records for instances in this EC2 region.
func (cache *EC2Cache) Records() map[Key][]*Record {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	return cache.records
}

// setRecords updates the cache with a new set of Records
func (cache *EC2Cache) setRecords(records map[Key][]*Record) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.records = records
}

func (cache *EC2Cache) refresh() {
	result, err := cache.Instances(nil, nil)
	validUntil := time.Now().Add(TTL)

	if err != nil {
		log.Println("Error: " + err.Error())
		return
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
	log.Printf("INFO: loaded records for %d instances", count)
	cache.setRecords(records)
}

// Lookup a node in the Cache either by Name or Role.
func (cache *EC2Cache) Lookup(tag LookupTag, value string) []*Record {
	return cache.Records()[Key{tag, value}]
}

func (record *Record) TTL(now time.Time) time.Duration {
	if now.After(record.ValidUntil) {
		return 10 * time.Second
	}
	return record.ValidUntil.Sub(now)
}
