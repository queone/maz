package maz

import (
	"fmt"
	"path/filepath"

	"github.com/queone/utl"
)

// Prints Azure AD role definition object in YAML-like format
func PrintAdRole(x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}

	// Print the most important attributes first
	list := []string{"id", "displayName", "description"}
	for _, i := range list {
		v := utl.Str(x[i])
		if v != "" { // Only print non-null attributes
			fmt.Printf("%s: %s\n", utl.Blu(i), utl.Gre(v))
		}
	}

	// Commenting this out for now. Too chatty. User can just run '-adj' to see full list of perms.
	// // List permissions
	// if x["rolePermissions"] != nil {
	// 	rolePerms := x["rolePermissions"].([]interface{})
	// 	if len(rolePerms) > 0 {
	// 		// Unclear why rolePermissions is a list instead of the single entry that it usually is
	// 		perms := rolePerms[0].(map[string]interface{})
	// 		if perms["allowedResourceActions"] != nil && len(perms["allowedResourceActions"].([]interface{})) > 0 {
	// 			fmt.Printf("permissions:\n")
	// 			for _, i := range perms["allowedResourceActions"].([]interface{}) {
	// 				fmt.Printf("  %s\n", utl.Str(i))
	// 			}
	// 		}
	// 	}
	// }

	// Print assignments
	// https://learn.microsoft.com/en-us/azure/active-directory/roles/view-assignments
	params := map[string]string{
		"$filter": "roleDefinitionId eq '" + utl.Str(x["templateId"]) + "'",
		"$expand": "principal",
	}
	url := ConstMgUrl + "/v1.0/roleManagement/directory/roleAssignments"
	r, statusCode, _ := ApiGet(url, z, params)
	if statusCode == 200 && r != nil && r["value"] != nil {
		assignments := r["value"].([]interface{})
		if len(assignments) > 0 {
			fmt.Printf(utl.Blu("assignments") + ":\n")
			//utl.PrintJsonColor(assignments)
			for _, i := range assignments {
				m := i.(map[string]interface{})
				scope := utl.Str(m["directoryScopeId"])
				// TODO: Find out how to get/print the scope displayName?
				mPrinc := m["principal"].(map[string]interface{})
				pName := utl.Str(mPrinc["displayName"])
				pType := utl.LastElem(utl.Str(mPrinc["@odata.type"]), ".")
				fmt.Printf("  %-50s  %-10s  %s\n", utl.Gre(pName), utl.Gre(pType), utl.Gre(scope))
			}
		}
	}

	// Print members of this role
	// See https://github.com/microsoftgraph/microsoft-graph-docs/blob/main/api-reference/v1.0/api/directoryrole-list-members.md
	// TODO: Fix 404 below for custom groups
	//   Resource '<custom role UUID>' does not exist or one of its queried reference-property objects are not present.
	url = ConstMgUrl + "/v1.0/directoryRoles(roleTemplateId='" + utl.Str(x["templateId"]) + "')/members"
	r, statusCode, _ = ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil {
		members := r["value"].([]interface{})
		if len(members) > 0 {
			fmt.Printf(utl.Blu("members") + ":\n")
			for _, i := range members {
				m := i.(map[string]interface{})
				id := utl.Gre(utl.Str(m["id"]))
				upn := utl.Gre(utl.Str(m["userPrincipalName"]))
				name := utl.Gre(utl.Str(m["displayName"]))
				fmt.Printf("  %s  %-40s   %s\n", id, upn, name)
			}
		}
	}
}

// Returns count of Azure AD directory role entries in local cache file
func AdRolesCountLocal(z Bundle) int64 {
	var cachedList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_directoryRoles."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList = rawList.([]interface{})
			return int64(len(cachedList))
		}
	}
	return 0
}

// Returns count of Azure AD directory role entries in current tenant
func AdRolesCountAzure(z Bundle) int64 {
	// Note that endpoint "/v1.0/directoryRoles" is for Activated AD roles, so it wont give us
	// the full count of all AD roles. Also, the actual role definitions, with what permissions
	// each has is at endpoint "/v1.0/roleManagement/directory/roleDefinitions", but because
	// we only care about their count it is easier to just call end point
	// "/v1.0/directoryRoleTemplates" which is a quicker API call and has the accurate count.
	// It's not clear why MSFT makes this so darn confusing.
	url := ConstMgUrl + "/v1.0/directoryRoleTemplates"
	r, _, _ := ApiGet(url, z, nil)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	if r["value"] != nil {
		return int64(len(r["value"].([]interface{})))
	}
	return 0
}

// Gets all AD roles matching on 'filter'. Returns entire list if filter is empty ""
func GetMatchingAdRoles(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_directoryRoles."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstMgCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstMgCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzAdRoles(z, true)
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
		// Match against relevant strings within AD role JSON object (Note: Not all attributes are maintained)
		if !utl.ItemInList(id, ids) && utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
			ids = append(ids, id)
		}
	}
	return matchingList
}

// Gets all directory role definitions from Azure and sync to local cache. Shows progress if verbose = true
func GetAzAdRoles(z Bundle, verbose bool) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_directoryRoles."+ConstCacheFileExtension)

	// There's no API delta options for this object (too short a list?), so just one call

	url := ConstMgUrl + "/beta/roleManagement/directory/roleDefinitions"
	r, _, _ := ApiGet(url, z, nil)
	if r["value"] == nil {
		return nil
	}
	list = r["value"].([]interface{})
	utl.SaveFileJsonGzip(list, cacheFile) // Update the local cache
	return list
}

// Gets Azure AD role definition by Object UUID, with all attributes
func GetAzAdRoleByUuid(uuid string, z Bundle) map[string]interface{} {
	// Note that role definitions are under a different area, until they are activated
	baseUrl := ConstMgUrl + "/beta/roleManagement/directory/roleDefinitions"
	selection := "?$select=*"
	url := baseUrl + "/" + uuid + selection
	r, _, _ := ApiGet(url, z, nil)
	return r
}
