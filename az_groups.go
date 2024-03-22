package maz

import (
	"fmt"
	"path/filepath"

	"github.com/queone/utl"
)

// Prints management group object in YAML-like format
func PrintMgGroup(x map[string]interface{}) {
	if x == nil {
		return
	}
	xProp := x["properties"].(map[string]interface{})
	fmt.Printf("%-12s: %s\n", utl.Blu("id"), utl.Gre(utl.Str(x["name"])))
	fmt.Printf("%-12s: %s\n", utl.Blu("displayName"), utl.Gre(utl.Str(xProp["displayName"])))
	fmt.Printf("%-12s: %s\n", utl.Blu("type"), utl.Gre(MgType(utl.Str(x["type"]))))
}

// Returns ARM object type based on long string
func MgType(typeIn string) string {
	switch typeIn {
	case "Microsoft.Management/managementGroups":
		return "ManagementGroup"
	case "Microsoft.Management/managementGroups/subscriptions", "/subscriptions":
		return "Subscription"
	default:
		return "??"
	}
}

// Returns count of management group objects in local cache file
func MgGroupCountLocal(z Bundle) int64 {
	var cachedList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_managementGroups."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList = rawList.([]interface{})
			return int64(len(cachedList))
		}
	}
	return 0
}

// Returns count of management groups in Azure
func MgGroupCountAzure(z Bundle) int64 {
	list := GetAzMgGroups(z)
	return int64(len(list))
}

// Returns id:name map of management groups
func GetIdMapMgGroups(z Bundle) (nameMap map[string]string) {
	nameMap = make(map[string]string)
	mgGroups := GetMatchingMgGroups("", false, z) // false = don't force a call to Azure
	// By not forcing an Azure call we're opting for cache speed over id:name map accuracy
	for _, i := range mgGroups {
		x := i.(map[string]interface{})
		nameMap[utl.Str(x["id"])] = utl.Str(x["name"])
	}
	return nameMap
}

// Gets all Azure management groups matching on 'filter'. Returns entire list if filter is empty ""
func GetMatchingMgGroups(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_managementGroups."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstAzCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstAzCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzMgGroups(z)
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
		// Match against relevant strings within managementGroups JSON object (Note: Not all attributes are maintained)
		if utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
		}
	}
	return matchingList
}

// Gets all management groups in current Azure tenant, and saves them to local cache file
func GetAzMgGroups(z Bundle) (list []interface{}) {
	list = nil                                               // We have to zero it out
	params := map[string]string{"api-version": "2020-05-01"} // managementGroups
	url := ConstAzUrl + "/providers/Microsoft.Management/managementGroups"
	r, _, _ := ApiGet(url, z, params)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	if r != nil && r["value"] != nil {
		objects := r["value"].([]interface{})
		list = append(list, objects...)
	}
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_managementGroups."+ConstCacheFileExtension)
	utl.SaveFileJsonGzip(list, cacheFile) // Update the local cache
	return list
}

// Recursively print management groups and all its children MGs and subscriptions
func PrintMgChildren(indent int, children []interface{}) {
	for _, i := range children {
		child := i.(map[string]interface{})
		Name := utl.Str(child["displayName"])
		Type := MgType(utl.Str(child["type"]))
		if Name == "Access to Azure Active Directory" && Type == "Subscription" {
			continue // Skip legacy subscriptions. We don't care
		}
		fmt.Printf("%*s", indent, " ") // Space padded indent
		padding := 38 - indent
		if padding < 12 {
			padding = 12
		}
		colorName := utl.Blu(utl.PostSpc(Name, padding))
		childName := utl.Gre(utl.PostSpc(utl.Str(child["name"]), 38))
		fmt.Printf("%s%s%s\n", colorName, childName, utl.Gre(Type))
		if child["children"] != nil {
			descendants := child["children"].([]interface{})
			PrintMgChildren(indent+4, descendants)
			// Using recursion here to print additional children
		}
	}
}

// Gets current tenant management group tree, and recursively calls function
// PrintMgChildren() to print the hierarchy
func PrintMgTree(z Bundle) {
	url := ConstAzUrl + "/providers/Microsoft.Management/managementGroups/" + z.TenantId
	params := map[string]string{
		"api-version": "2020-05-01", // managementGroups
		"$expand":     "children",
		"$recurse":    "true",
	}
	r, _, _ := ApiGet(url, z, params)
	ApiErrorCheck("GET", url, utl.Trace(), r) // DEBUG: Need to see when this is failing for some users
	if r["properties"] != nil {
		// Print everything under the hierarchy
		Prop := r["properties"].(map[string]interface{})
		name := utl.Blu(utl.PostSpc(utl.Str(Prop["displayName"]), 38))
		tenantId := utl.Blu(utl.PostSpc(utl.Str(Prop["tenantId"]), 38))
		fmt.Printf("%s%s%s\n", name, tenantId, utl.Blu("TENANT"))
		if Prop["children"] != nil {
			children := Prop["children"].([]interface{})
			PrintMgChildren(4, children)
		}
	}
}
