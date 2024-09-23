// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Intel Corporation, or its subsidiaries.
// Copyright (C) 2023 Nordix Foundation.

// Package linuxgeneralmodule is the main package of the application

package utils

import (
	"fmt"
	"log"
	"reflect"
)

// IDPool structure
/*  Helper class for uniquely assigning IDs from a specified integer set (e.g. a
#   range) to keys. IDs are assigned (or read) with GetID(key) and returned back
#   into the pool with ReleaseID(key). The IDPool remembers a once-assigned ID
#   for keys so that the same ID is assigned for a key. Only when the pool runs
#   out of unassigned keys, it will recycle released ids and assign them to new
#   keys.
#   Optionally, the IDPool supports reference tracking for key/ID pairs. Clients
#   can provide a unique reference when fetching and releasing an ID for a key
#   to support multiple independent clients.
#   The pool will only release the ID for the key, when the last client has the
#   released the ID with its reference. When a reference is specified in GetID()
#   and ReleaseID() the IDPool returns the current number of reference for the
#   ID so that a caller knows when an ID was newly assigned (ref_count 1) or
#   finally released (ref_count 0).
*/
type IDPool struct {
	name        string                 // Name of pool
	unusedIDs   []uint32               // Yet unused IDs in pool Available ids
	idsInUse    map[interface{}]uint32 // Mapping key: id for currently assigned ids
	idsForReuse map[interface{}]uint32 // Mapping key: id for previously assigned ids
	refs        map[uint32][]interface{}
	size        int // Size of the pool
}

// IDPoolInit initialize mod ptr pool
func IDPoolInit(name string, min uint32, max uint32) IDPool {
	var pool IDPool
	pool.name = name
	var index int
	pool.unusedIDs = make([]uint32, (max-min)+1)
	if min > 0 {
		for value := max; value >= min; value-- {
			pool.unusedIDs[index] = value
			index++
		}
	} else {
		return IDPool{}
	}
	pool.size = len(pool.unusedIDs)
	pool.idsInUse = make(map[interface{}]uint32)
	pool.idsForReuse = make(map[interface{}]uint32)
	pool.refs = make(map[uint32][]interface{})
	return pool
}

// GetPoolStatus get status of a pool
func (ip *IDPool) GetPoolStatus() string {
	str := fmt.Sprintf("name=%s\n Inuse=%+v\n Refs=%+v\n Forreuse=%+v\n Unused=%+v\n", ip.name, ip.idsInUse, ip.refs, ip.idsForReuse, ip.unusedIDs)
	return str
}

func (ip *IDPool) assignid(key interface{}) uint32 {
	// Check if there was an id assigned for that key earlier
	var id uint32
	ok := ip.idsForReuse[key]
	if ok != 0 {
		// Re-use the old id
		delete(ip.idsForReuse, key)
	} else {
		if len(ip.unusedIDs) != 0 {
			// Pick an unused id
			id = ip.unusedIDs[len(ip.unusedIDs)-1]
			ip.unusedIDs = ip.unusedIDs[0 : len(ip.unusedIDs)-1]
		} else {
			if len(ip.idsForReuse) != 0 {
				// Pick one of the ids earlier used for another key
				for oldKey := range ip.idsForReuse {
					delete(ip.idsForReuse, oldKey)
					break
				}
			} else {
				log.Printf("IDPool: Failed to allocate id for %+v. No free ids in pool.", key)
				return 0
			}
		}
	}
	// Store the assigned id, if any
	if id != 0 {
		ip.idsInUse[key] = id
	}
	return id
}

// GetID get the mod ptr id from pool
func (ip *IDPool) GetID(key interface{}, ref interface{}) (uint32, uint32) {
	var id uint32
	ok := ip.idsInUse[key]
	if ok == 0 {
		id = ip.assignid(key)
		if id == 0 {
			return 0, 0
		}
	}
	id = ok
	if ref != nil {
		log.Printf("IDPool: GetID  Assigning key : %+v , id  %+v for ref %v", id, key, ref)
		if reflect.ValueOf(ip.refs[id]).IsZero() {
			ip.refs[id] = make([]interface{}, 0)
		}
		ip.refs[id] = append(ip.refs[id], ref)
		return id, uint32(len(ip.refs[id]))
	}
	log.Printf("IDPool: GetID Assigning id %v for key %v and ref %v", id, key, ref)
	return id, uint32(0)
}

func deleteRef(refSet []interface{}, ref interface{}) []interface{} {
	var i uint32
	for index, value := range refSet {
		if value == ref {
			i = uint32(index)
			break
		}
	}
	return append(refSet[:i], refSet[i+1:]...)
}

// ReleaseID get the reference id
func (ip *IDPool) ReleaseID(key interface{}, ref interface{}) (uint32, uint32) {
	log.Printf("IDPool:ReleaseID  Releasing id for key %v", key)
	id := ip.idsInUse[key]
	if ref == nil {
		log.Printf("No id to release for key %v", key)
		return 0, 0
	}
	refSet := ip.refs[id]
	if !reflect.ValueOf(refSet).IsZero() && !reflect.ValueOf(ref).IsZero() {
		refSet = deleteRef(refSet, ref)
	}
	if refSet != nil {
		log.Printf("IDPool:ReleaseID Id %v has been released", id)
		delete(ip.idsInUse, key)
		if refSet != nil {
			delete(ip.refs, id)
		}
		// Store released id for future reassignment
		ip.idsForReuse[key] = id
	} else {
		log.Printf("IDPool:ReleaseID Keep id:%+v remaining references %+v", id, len(refSet))
	}
	if ref != nil {
		return id, uint32(len(refSet))
	}
	return id, uint32(0)
}
