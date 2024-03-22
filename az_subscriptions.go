package maz

import (
	"fmt"
	"path/filepath"

	"github.com/queone/utl"
)

// Prints subscription object in YAML-like format
func PrintSubscription(x map[string]interface{}) {
	if x == nil {
		return
	}
	list := []string{"subscriptionId", "displayName", "state", "tenantId"}
	for _, i := range list {
		v := utl.Str(x[i])
		if v != "" { // Only print non-null attributes
			fmt.Printf("%s: %s\n", utl.Blu(i), utl.Gre(v))
		}
	}
}

// Returns count of all subscriptions in local cache file
func SubsCountLocal(z Bundle) int64 {
	var cachedList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_subscriptions."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList = rawList.([]interface{})
			return int64(len(cachedList))
		}
	}
	return 0
}

// Returns count of all subscriptions in current Azure tenant
func SubsCountAzure(z Bundle) int64 {
	list := GetAzSubscriptions(z)
	return int64(len(list))
}

// Gets all subscription full IDs, i.e. "/subscriptions/UUID", which are commonly
// used as scopes for Azure resource RBAC role definitions and assignments
func GetAzSubscriptionsIds(z Bundle) (scopes []string) {
	scopes = nil
	subscriptions := GetAzSubscriptions(z)
	for _, i := range subscriptions {
		x := i.(map[string]interface{})
		// Skip disabled and legacy subscriptions
		displayName := utl.Str(x["displayName"])
		state := utl.Str(x["state"])
		if state != "Enabled" || displayName == "Access to Azure Active Directory" {
			continue
		}
		subId := utl.Str(x["id"])
		scopes = append(scopes, subId)
	}
	return scopes
}

// Returns id:name map of all subscriptions
func GetIdMapSubs(z Bundle) (nameMap map[string]string) {
	nameMap = make(map[string]string)
	roleDefs := GetMatchingSubscriptions("", false, z) // false = don't force a call to Azure
	// By not forcing an Azure call we're opting for cache speed over id:name map accuracy
	for _, i := range roleDefs {
		x := i.(map[string]interface{})
		if x["subscriptionId"] != nil && x["displayName"] != nil {
			nameMap[utl.Str(x["subscriptionId"])] = utl.Str(x["displayName"])
		}
	}
	return nameMap
}

// Gets all Azure subscriptions matching on 'filter'. Returns entire list if filter is empty ""
func GetMatchingSubscriptions(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_subscriptions."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstAzCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstAzCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzSubscriptions(z)
	} else {
		// Use local cache for all other conditions
		list = GetCachedObjects(cacheFile)
	}

	if filter == "" {
		return list
	}
	var matchingList []interface{} = nil
	for _, i := range list { // Parse every object
		x := i.(map[string]interface{})
		// Match against relevant strings within subscription JSON object (Note: Not all attributes are maintained)
		if utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
		}
	}
	return matchingList
}

// Gets all subscription in current Azure tenant, and saves them to local cache file
func GetAzSubscriptions(z Bundle) (list []interface{}) {
	list = nil                                               // We have to zero it out
	params := map[string]string{"api-version": "2022-09-01"} // subscriptions
	url := ConstAzUrl + "/subscriptions"
	r, _, _ := ApiGet(url, z, params)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	if r != nil && r["value"] != nil {
		objects := r["value"].([]interface{})
		list = append(list, objects...)
	}
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_subscriptions."+ConstCacheFileExtension)
	utl.SaveFileJsonGzip(list, cacheFile) // Update the local cache
	return list
}

// Gets specific Azure subscription by Object UUID
func GetAzSubscriptionByUuid(uuid string, z Bundle) map[string]interface{} {
	params := map[string]string{"api-version": "2022-09-01"} // subscriptions
	url := ConstAzUrl + "/subscriptions/" + uuid
	r, _, _ := ApiGet(url, z, params)
	//ApiErrorCheck("GET", url, utl.Trace(), r) // Commented out to do this quietly. Use for DEBUGging
	return r
}
