package maz

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/queone/utl"
)

// Prints RBAC role definition object in YAML-like format
func PrintRoleAssignment(x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}
	if x["name"] != nil {
		fmt.Printf("%s: %s\n", utl.Blu("id"), utl.Gre(utl.Str(x["name"])))
	}
	if x["properties"] != nil {
		fmt.Println(utl.Blu("properties") + ":")
	} else {
		fmt.Println("  < Missing properties? What's going? >")
	}

	xProp := x["properties"].(map[string]interface{})

	roleNameMap := GetIdMapRoleDefs(z) // Get all role definition id:name pairs
	roleId := utl.LastElem(utl.Str(xProp["roleDefinitionId"]), "/")
	comment := "# Role \"" + roleNameMap[roleId] + "\""
	fmt.Printf("  %s: %s  %s\n", utl.Blu("roleDefinitionId"), utl.Gre(roleId), comment)

	var principalNameMap map[string]string = nil
	pType := utl.Str(xProp["principalType"])
	switch pType {
	case "Group":
		principalNameMap = GetIdMapGroups(z) // Get all users id:name pairs
	case "User":
		principalNameMap = GetIdMapUsers(z) // Get all users id:name pairs
	case "ServicePrincipal":
		principalNameMap = GetIdMapSps(z) // Get all SPs id:name pairs
	default:
		pType = "SomeObject"
	}
	principalId := utl.Str(xProp["principalId"])
	pName := principalNameMap[principalId]
	if pName == "" {
		pName = "???"
	}
	comment = "# " + pType + " \"" + pName + "\""
	fmt.Printf("  %s: %s  %s\n", utl.Blu("principalId"), utl.Gre(principalId), comment)

	subNameMap := GetIdMapSubs(z) // Get all subscription id:name pairs
	scope := utl.Str(xProp["scope"])
	if scope == "" {
		scope = utl.Str(xProp["Scope"])
	} // Account for possibly capitalized key
	cScope := utl.Blu("scope")
	if strings.HasPrefix(scope, "/subscriptions") {
		split := strings.Split(scope, "/")
		subName := subNameMap[split[2]]
		comment = "# Sub = " + subName
		fmt.Printf("  %s: %s  %s\n", cScope, utl.Gre(scope), comment)
	} else if scope == "/" {
		comment = "# Entire tenant"
		fmt.Printf("  %s: %s  %s\n", cScope, utl.Gre(scope), comment)
	} else {
		fmt.Printf("  %s: %s\n", cScope, utl.Gre(scope))
	}
}

// Prints a human-readable report of all RBAC role assignments
func PrintRoleAssignmentReport(z Bundle) {
	roleNameMap := GetIdMapRoleDefs(z) // Get all role definition id:name pairs
	subNameMap := GetIdMapSubs(z)      // Get all subscription id:name pairs
	groupNameMap := GetIdMapGroups(z)  // Get all users id:name pairs
	userNameMap := GetIdMapUsers(z)    // Get all users id:name pairs
	spNameMap := GetIdMapSps(z)        // Get all SPs id:name pairs

	assignments := GetAzRoleAssignments(z, false)
	for _, i := range assignments {
		x := i.(map[string]interface{})
		xProp := x["properties"].(map[string]interface{})
		Rid := utl.LastElem(utl.Str(xProp["roleDefinitionId"]), "/")
		principalId := utl.Str(xProp["principalId"])
		Type := utl.Str(xProp["principalType"])
		pName := "ID-Not-Found"
		switch Type {
		case "Group":
			pName = groupNameMap[principalId]
		case "User":
			pName = userNameMap[principalId]
		case "ServicePrincipal":
			pName = spNameMap[principalId]
		}

		Scope := utl.Str(xProp["scope"])
		if strings.HasPrefix(Scope, "/subscriptions") {
			// Replace sub ID to name
			split := strings.Split(Scope, "/")
			// Map subscription Id to its name + the rest of the resource path
			Scope = subNameMap[split[2]] + " " + strings.Join(split[3:], "/")
		}
		Scope = strings.TrimSpace(Scope)

		fmt.Printf("\"%s\",\"%s\",\"%s\",\"%s\"\n", roleNameMap[Rid], pName, Type, Scope)
	}
}

// Creates an RBAC role assignment as defined by give x object
func CreateAzRoleAssignment(x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}
	xProp := x["properties"].(map[string]interface{})
	roleDefinitionId := utl.LastElem(utl.Str(xProp["roleDefinitionId"]), "/") // Note we only care about the UUID
	principalId := utl.Str(xProp["principalId"])
	scope := utl.Str(xProp["scope"])
	if scope == "" {
		scope = utl.Str(xProp["Scope"]) // Account for possibly capitalized key
	}
	if roleDefinitionId == "" || principalId == "" || scope == "" {
		utl.Die("Specfile is missing required attributes. Need at least:\n\n" +
			"properties:\n" +
			"    roleDefinitionId: <UUID or fully_qualified_roleDefinitionId>\n" +
			"    principalId: <UUID>\n" +
			"    scope: <resource_path_scope>\n\n" +
			"See script '-k*' options to create properly formatted sample files.\n")
	}

	// Note, there is no need to pre-check if assignment exists, since call will simply let us know
	newUuid := uuid.New().String() // Generate a new global UUID in string format
	payload := map[string]interface{}{
		"properties": map[string]string{
			"roleDefinitionId": "/providers/Microsoft.Authorization/roleDefinitions/" + roleDefinitionId,
			"principalId":      principalId,
		},
	}
	params := map[string]string{"api-version": "2022-04-01"} // roleAssignments
	url := ConstAzUrl + scope + "/providers/Microsoft.Authorization/roleAssignments/" + newUuid
	r, statusCode, _ := ApiPut(url, z, payload, params)
	//ApiErrorCheck("PUT", url, utl.Trace(), r)
	if statusCode == 200 || statusCode == 201 {
		utl.PrintYaml(r)
	} else {
		e := r["error"].(map[string]interface{})
		fmt.Println(e["message"].(string))
	}
}

// Deletes an RBAC role assignment by its fully qualified object Id
// Example of a fully qualified Id string (note it's one long line):
//
//	/providers/Microsoft.Management/managementGroups/33550b0b-2929-4b4b-adad-cccc66664444 \
//	  /providers/Microsoft.Authorization/roleAssignments/5d586a7b-3f4b-4b5c-844a-3fa8efe49ab3
func DeleteAzRoleAssignmentByFqid(fqid string, z Bundle) map[string]interface{} {
	params := map[string]string{"api-version": "2022-04-01"} // roleAssignments
	url := ConstAzUrl + fqid
	r, statusCode, _ := ApiDelete(url, z, params)
	//ApiErrorCheck("DELETE", url, utl.Trace(), r)
	if statusCode != 200 {
		if statusCode == 204 {
			fmt.Println("Role assignment already deleted or does not exist. Give Azure a minute to flush it out.")
		} else {
			e := r["error"].(map[string]interface{})
			fmt.Println(e["message"].(string))
		}
	}
	return nil
}

// Retrieves count of all role assignment objects in local cache file
func RoleAssignmentsCountLocal(z Bundle) int64 {
	var cachedList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_roleAssignments."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList = rawList.([]interface{})
			return int64(len(cachedList))
		}
	}
	return 0
}

// Calculates count of all role assignment objects in Azure
func RoleAssignmentsCountAzure(z Bundle) int64 {
	list := GetAzRoleAssignments(z, false) // false = quiet
	return int64(len(list))
}

// Gets all RBAC role assignments matching on 'filter'. Return entire list if filter is empty ""
func GetMatchingRoleAssignments(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_roleAssignments."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstAzCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstAzCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzRoleAssignments(z, true)
	} else {
		// Use local cache for all other conditions
		list = GetCachedObjects(cacheFile)
	}

	if filter == "" {
		return list
	}
	var matchingList []interface{} = nil
	roleNameMap := GetIdMapRoleDefs(z) // Get all role definition id:name pairs
	for _, i := range list {           // Parse every object
		x := i.(map[string]interface{})
		// Match against relevant strings within roleAssigment JSON object (Note: Not all attributes are maintained)
		xProp := x["properties"].(map[string]interface{})
		roleId := utl.Str(xProp["roleDefinitionId"])
		roleName := roleNameMap[utl.LastElem(roleId, "/")]
		if utl.SubString(roleName, filter) || utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
		}
	}
	return matchingList
}

// Gets all role assignments objects in current Azure tenant and save them to local cache file.
// Option to be verbose (true) or quiet (false), since it can take a while.
// References:
//
//	https://learn.microsoft.com/en-us/azure/role-based-access-control/role-assignments-list-rest
//	https://learn.microsoft.com/en-us/rest/api/authorization/role-assignments/list-for-subscription
func GetAzRoleAssignments(z Bundle, verbose bool) (list []interface{}) {
	list = nil             // We have to zero it out
	var uniqueIds []string // Keep track of assignment objects
	k := 1                 // Track number of API calls to provide progress

	var mgGroupNameMap, subNameMap map[string]string
	if verbose {
		mgGroupNameMap = GetIdMapMgGroups(z)
		subNameMap = GetIdMapSubs(z)
	}

	scopes := GetAzRbacScopes(z)                             // Get all scopes
	params := map[string]string{"api-version": "2022-04-01"} // roleAssignments
	for _, scope := range scopes {
		url := ConstAzUrl + scope + "/providers/Microsoft.Authorization/roleAssignments"
		r, _, _ := ApiGet(url, z, params)
		if r != nil && r["value"] != nil {
			objectsUnderThisScope := r["value"].([]interface{})
			count := 0
			for _, i := range objectsUnderThisScope {
				x := i.(map[string]interface{})
				uuid := utl.Str(x["name"])
				if utl.ItemInList(uuid, uniqueIds) {
					continue // Skip this repeated one. This can happen due to inherited nesting
				}
				uniqueIds = append(uniqueIds, uuid) // Keep track of the UUIDs we are seeing
				list = append(list, x)
				count++
			}
			if verbose && count > 0 {
				scopeName := scope
				if strings.HasPrefix(scope, "/providers") {
					scopeName = mgGroupNameMap[scope]
				} else if strings.HasPrefix(scope, "/subscriptions") {
					scopeName = subNameMap[utl.LastElem(scope, "/")]
				}
				fmt.Printf("API call %4d: %5d objects under %s\n", k, count, scopeName)
			}
		}
		k++
	}
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_roleAssignments."+ConstCacheFileExtension)
	utl.SaveFileJsonGzip(list, cacheFile) // Update the local cache
	return list
}

// Gets Azure resource RBAC role assignment object by matching given objects: roleId, principalId,
// and scope (the 3 parameters which make a role assignment unique)
func GetAzRoleAssignmentByObject(x map[string]interface{}, z Bundle) (y map[string]interface{}) {
	// First, make sure x is a searchable role assignment object
	if x == nil {
		return nil
	}
	xProp := x["properties"].(map[string]interface{})
	if xProp == nil {
		return nil
	}

	xRoleDefinitionId := utl.LastElem(utl.Str(xProp["roleDefinitionId"]), "/")
	xPrincipalId := utl.Str(xProp["principalId"])
	xScope := utl.Str(xProp["scope"])
	if xScope == "" {
		xScope = utl.Str(xProp["Scope"]) // Account for possibly capitalized key
	}
	if xScope == "" || xPrincipalId == "" || xRoleDefinitionId == "" {
		return nil
	}

	// Get all role assignments for xPrincipalId under xScope
	params := map[string]string{
		"api-version": "2022-04-01", // roleAssignments
		"$filter":     "principalId eq '" + xPrincipalId + "'",
	}
	url := ConstAzUrl + xScope + "/providers/Microsoft.Authorization/roleAssignments"
	r, _, _ := ApiGet(url, z, params)
	//ApiErrorCheck("GET", url, utl.Trace(), r)
	if r != nil && r["value"] != nil {
		results := r["value"].([]interface{})
		//fmt.Println(len(results))
		for _, i := range results {
			y = i.(map[string]interface{})
			yProp := y["properties"].(map[string]interface{})
			yScope := utl.Str(yProp["scope"])
			yRoleDefinitionId := utl.LastElem(utl.Str(yProp["roleDefinitionId"]), "/")
			if yScope == xScope && yRoleDefinitionId == xRoleDefinitionId {
				return y // As soon as we find it
			}
		}
	}
	return nil // If we get here, we didn't fine it, so return nil
}

// Gets RBAC role assignment by its Object UUID. Unfortunately we have to iterate
// through the entire tenant scope hierarchy, which can take time.
func GetAzRoleAssignmentByUuid(uuid string, z Bundle) map[string]interface{} {
	scopes := GetAzRbacScopes(z)
	params := map[string]string{"api-version": "2022-04-01"} // roleAssignments
	for _, scope := range scopes {
		url := ConstAzUrl + scope + "/providers/Microsoft.Authorization/roleAssignments"
		r, _, _ := ApiGet(url, z, params)
		if r != nil && r["value"] != nil {
			assignmentsUnderThisScope := r["value"].([]interface{})
			for _, i := range assignmentsUnderThisScope {
				x := i.(map[string]interface{})
				if utl.Str(x["name"]) == uuid {
					return x // Return as soon as we find a match
				}
			}
		}
	}
	return nil
}
