package maz

import (
	"github.com/queone/utl"
)

// Selects JSON object with given ID from slice
func SelectObject(id string, objSet []interface{}) (x map[string]interface{}) {
	for _, obj := range objSet {
		x = obj.(map[string]interface{})
		objId := utl.Str(x["id"])
		if id == objId {
			return x
		}
	}
	return nil
}

// Builds JSON mergeSet from deltaSet, and builds and returns the list of deleted IDs
func NormalizeCache(baseSet, deltaSet []interface{}) (list []interface{}) {
	var deletedIds []string
	var uniqueIds []string
	var mergeSet []interface{} = nil
	for _, i := range deltaSet {
		x := i.(map[string]interface{})
		id := utl.Str(x["id"])
		if x["@removed"] == nil && x["members@delta"] == nil {
			// Only add to mergeSet if '@remove' and 'members@delta' are missing
			if !utl.ItemInList(id, uniqueIds) {
				// Only add if it's unique
				mergeSet = append(mergeSet, x)
				uniqueIds = append(uniqueIds, id) // Track unique IDs
			}
		} else {
			deletedIds = append(deletedIds, id)
		}
	}

	// Remove recently deleted entries (deletedIs) from baseSet
	list = nil
	var baseIds []string = nil // Track all the IDs in the base cache set
	for _, i := range baseSet {
		x := i.(map[string]interface{})
		id := utl.Str(x["id"])
		if utl.ItemInList(id, deletedIds) {
			continue
		}
		list = append(list, x)
		baseIds = append(baseIds, id)
	}

	// Merge new entries in deltaSet into baseSet
	var duplicates []interface{} = nil
	var duplicateIds []string = nil
	for _, obj := range mergeSet {
		x := obj.(map[string]interface{})
		id := utl.Str(x["id"])
		if utl.ItemInList(id, baseIds) {
			duplicates = append(duplicates, x)
			duplicateIds = append(duplicateIds, id)
			continue // Skip duplicates (these are updates)
		}
		list = append(list, x) // Merge all others (these are new entries)
	}

	// Merge updated entries in deltaSet into baseSet
	list2 := list
	list = nil
	for _, obj := range list2 {
		x := obj.(map[string]interface{})
		id := utl.Str(x["id"])
		if !utl.ItemInList(id, duplicateIds) {
			// If this object is not a duplicate, add it to our growing list
			list = append(list, x)
		} else {
			// Merge object updates, then add it to our growing list
			y := SelectObject(id, duplicates)
			x = utl.MergeObjects(x, y)
			list = append(list, x)
		}
	}

	return list
}
