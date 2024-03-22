package maz

import (
	"fmt"
	"path/filepath"

	"github.com/queone/utl"
)

// Prints user object in YAML-like format
func PrintUser(x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}
	id := utl.Str(x["id"])

	// Print the primary keys first
	keys := []string{"id", "displayName", "userPrincipalName", "onPremisesSamAccountName", "onPremisesDomainName"}
	for _, i := range keys {
		if v := utl.Str(x[i]); v != "" { // Print only non-empty keys
			fmt.Printf("%s: %s\n", utl.Blu(i), utl.Gre(v))
		}
	}

	// Print app role assignment members and the specific role assigned
	//url := ConstMgUrl + "/v1.0/users/" + id + "/appRoleAssignments"
	url := ConstMgUrl + "/beta/users/" + id + "/appRoleAssignments"
	appRoleAssignments := GetAzAllPages(url, z)
	PrintAppRoleAssignmentsOthers(appRoleAssignments, z)

	// Print all groups and roles it is a member of
	url = ConstMgUrl + "/v1.0/users/" + id + "/transitiveMemberOf"
	r, statusCode, _ := ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil {
		memberOf := r["value"].([]interface{})
		PrintMemberOfs("g", memberOf)
	}
}

// Returns the number of entries in local cache file
func UsersCountLocal(z Bundle) int64 {
	var cachedList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_users."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList = rawList.([]interface{})
			return int64(len(cachedList))
		}
	}
	return 0
}

// Returns the number of entries in Azure tenant
func UsersCountAzure(z Bundle) int64 {
	z.MgHeaders["ConsistencyLevel"] = "eventual"
	url := ConstMgUrl + "/v1.0/users/$count"
	r, _, _ := ApiGet(url, z, nil)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	if r["value"] != nil {
		return r["value"].(int64) // Expected result is a single int64 value for the count
	}
	return 0
}

// Returns an id:name map of all users
func GetIdMapUsers(z Bundle) (nameMap map[string]string) {
	nameMap = make(map[string]string)
	users := GetMatchingUsers("", false, z) // false = don't force a call to Azure
	// By not forcing an Azure call we're opting for cache speed over id:name map accuracy
	for _, i := range users {
		x := i.(map[string]interface{})
		if x["id"] != nil && x["displayName"] != nil {
			nameMap[utl.Str(x["id"])] = utl.Str(x["displayName"])
		}
	}
	return nameMap
}

// Gets all users matching on 'filter'. Returns entire list if filter is empty ""
func GetMatchingUsers(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_users."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstMgCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstMgCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzUsers(z, true)
	} else {
		// Use local cache for all other conditions
		list = GetCachedObjects(cacheFile)
	}

	if filter == "" {
		return list
	}
	var matchingList []interface{} = nil
	var ids []string // Keep track of each unique objects to eliminate repeats
	for _, i := range list {
		x := i.(map[string]interface{})
		id := utl.Str(x["id"])
		// Match against relevant strings within user JSON object (Note: Not all attributes are maintained)
		if !utl.ItemInList(id, ids) && utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
			ids = append(ids, id)
		}
	}
	return matchingList
}

// Gets all users from Azure and sync to local cache. Show progress if verbose = true
func GetAzUsers(z Bundle, verbose bool) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_users."+ConstCacheFileExtension)
	deltaLinkFile := filepath.Join(z.ConfDir, z.TenantId+"_users_deltaLink."+ConstCacheFileExtension)

	baseUrl := ConstMgUrl + "/beta/users"
	// Get delta updates only if/when selection attributes are modified
	selection := "?$select=displayName,userPrincipalName,onPremisesSamAccountName"
	url := baseUrl + "/delta" + selection + "&$top=999"
	list = GetCachedObjects(cacheFile) // Get current cache
	if len(list) < 1 {
		// These are only needed on initial cache run
		z.MgHeaders["Prefer"] = "return=minimal" // Tells API to focus only on $select attributes deltas
		z.MgHeaders["deltaToken"] = "latest"
		// https://graph.microsoft.com/v1.0/users/delta?$deltatoken=latest
	}

	// Prep to do a delta query if it is possible
	var deltaLinkMap map[string]interface{} = nil
	if utl.FileUsable(deltaLinkFile) && utl.FileAge(deltaLinkFile) < (3660*24*27) && len(list) > 0 {
		// Note that deltaLink file age has to be within 30 days (we do 27)
		tmpVal, _ := utl.LoadFileJsonGzip(deltaLinkFile)
		deltaLinkMap = tmpVal.(map[string]interface{})
		url = utl.Str(utl.Str(deltaLinkMap["@odata.deltaLink"]))
		// Base URL is now the cached Delta Link URL
	}

	// Now go get Azure objects using the updated URL (either a full or a delta query)
	var deltaSet []interface{} = nil
	deltaSet, deltaLinkMap = GetAzObjects(url, z, verbose) // Run generic deltaSet retriever function

	// Save new deltaLink for future call, and merge newly acquired delta set with existing list
	utl.SaveFileJsonGzip(deltaLinkMap, deltaLinkFile)
	list = NormalizeCache(list, deltaSet) // Run our MERGE LOGIC with new delta set
	utl.SaveFileJsonGzip(list, cacheFile) // Update the local cache
	return list
}

// Gets Azure user object by Object UUID, with all attributes
func GetAzUserByUuid(uuid string, z Bundle) map[string]interface{} {
	baseUrl := ConstMgUrl + "/beta/users"
	selection := "?$select=*"
	url := baseUrl + "/" + uuid + selection
	r, _, _ := ApiGet(url, z, nil)
	return r
}
