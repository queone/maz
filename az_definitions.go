package maz

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/queone/utl"
)

// Prints role definition object in a YAML-like format
func PrintRoleDefinition(x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}
	if x["name"] != nil {
		fmt.Printf("%s: %s\n", utl.Blu("id"), utl.Gre(utl.Str(x["name"])))
	}
	if x["properties"] != nil {
		fmt.Println(utl.Blu("properties") + ":")
	} else {
		fmt.Println(utl.Red("  <Missing properties??>"))
	}

	xProp := x["properties"].(map[string]interface{})

	list := []string{"roleName", "description"}
	for _, i := range list {
		fmt.Printf("  %s: %s\n", utl.Blu(i), utl.Gre(utl.Str(xProp[i])))
	}

	fmt.Printf("  %s: ", utl.Blu("assignableScopes"))
	if xProp["assignableScopes"] == nil {
		fmt.Printf("[]\n")
	} else {
		fmt.Printf("\n")
		scopes := xProp["assignableScopes"].([]interface{})
		if len(scopes) > 0 {
			subNameMap := GetIdMapSubs(z) // Get all subscription id:name pairs
			for _, i := range scopes {
				if strings.HasPrefix(i.(string), "/subscriptions") {
					// Print subscription name as a comment at end of line
					subId := utl.LastElem(i.(string), "/")
					comment := "# " + subNameMap[subId]
					fmt.Printf("    - %s  %s\n", utl.Gre(utl.Str(i)), comment)
				} else {
					fmt.Printf("    - %s\n", utl.Gre(utl.Str(i)))
				}
			}
		} else {
			fmt.Println(utl.Red("    <Not an arrays??>\n"))
		}
	}

	fmt.Printf("  %s:\n", utl.Blu("permissions"))
	if xProp["permissions"] == nil {
		fmt.Println(utl.Red("    < No permissions?? >\n"))
	} else {
		permsSet := xProp["permissions"].([]interface{})
		if len(permsSet) == 1 {
			perms := permsSet[0].(map[string]interface{}) // Select the 1 expected single permission set

			fmt.Printf("    - " + utl.Blu("actions") + ":\n") // Note that this one is different, as it starts the YAML array with the dash '-'
			if perms["actions"] != nil {
				permsA := perms["actions"].([]interface{})
				if utl.GetType(permsA)[0] != '[' { // Open bracket character means it's an array list
					fmt.Println(utl.Red("        <Not an array??>\n"))
				} else {
					for _, i := range permsA {
						s := utl.StrSingleQuote(i) // Special function to lookout for leading '*' which must be single-quoted
						fmt.Printf("        - %s\n", utl.Gre(s))
					}
				}
			}

			fmt.Printf("      " + utl.Blu("notActions") + ":\n")
			if perms["notActions"] != nil {
				permsNA := perms["notActions"].([]interface{})
				if utl.GetType(permsNA)[0] != '[' {
					fmt.Println(utl.Red("        <Not an array??>\n"))
				} else {
					for _, i := range permsNA {
						s := utl.StrSingleQuote(i)
						fmt.Printf("        - %s\n", utl.Gre(s))
					}
				}
			}

			fmt.Printf("      " + utl.Blu("dataActions") + ":\n")
			if perms["dataActions"] != nil {
				permsDA := perms["dataActions"].([]interface{})
				if utl.GetType(permsDA)[0] != '[' {
					fmt.Println(utl.Red("        <Not an array??>\n"))
				} else {
					for _, i := range permsDA {
						s := utl.StrSingleQuote(i)
						fmt.Printf("        - %s\n", utl.Gre(s))
					}
				}
			}

			fmt.Printf("      " + utl.Blu("notDataActions") + ":\n")
			if perms["notDataActions"] != nil {
				permsNDA := perms["notDataActions"].([]interface{})
				if utl.GetType(permsNDA)[0] != '[' {
					fmt.Println(utl.Red("        <Not an array??>\n"))
				} else {
					for _, i := range permsNDA {
						s := utl.StrSingleQuote(i)
						fmt.Printf("        - %s\n", utl.Gre(s))
					}
				}
			}

		} else {
			fmt.Println(utl.Red("    <More than one set??>\n"))
		}
	}
}

// Creates or updates an RBAC role definition as defined by give x object
func UpsertAzRoleDefinition(force bool, x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}
	xProp := x["properties"].(map[string]interface{})
	xRoleName := utl.Str(xProp["roleName"])
	// Below two are required in the API body call, but we don't need to burden
	// the user with this requirement, and just update the values for them here.
	if xProp["type"] == nil {
		xProp["type"] = "CustomRole"
	}
	if xProp["description"] == nil {
		xProp["description"] = ""
	}
	xScopes := xProp["assignableScopes"].([]interface{})
	xScope1 := utl.Str(xScopes[0]) // For deployment, we'll use 1st scope
	var permSet []interface{} = nil
	if xProp["permissions"] != nil {
		permSet = xProp["permissions"].([]interface{})
	}
	if xProp == nil || xScopes == nil || xRoleName == "" || xScope1 == "" ||
		permSet == nil || len(permSet) < 1 {
		utl.Die("Specfile is missing required attributes. The bare minimum is:\n\n" +
			"properties:\n" +
			"  roleName: \"My Role Name\"\n" +
			"  assignableScopes:\n" +
			"    - /providers/Microsoft.Management/managementGroups/3f550b9f-8888-7777-ad61-111199992222\n" +
			"  permissions:\n" +
			"    - actions:\n\n" +
			"See script '-k*' options to create properly formatted sample files.\n")
	}

	roleId := ""
	existing := GetAzRoleDefinitionByName(xRoleName, z)
	if existing == nil {
		// Role definition doesn't exist, so we're creating a new one
		roleId = uuid.New().String() // Generate a new global UUID in string format
	} else {
		// Role exists, we'll prompt for update choice
		PrintRoleDefinition(existing, z)
		if !force {
			msg := utl.Yel("Role already exists! UPDATE it? y/n ")
			if utl.PromptMsg(msg) != 'y' {
				utl.Die("Aborted.\n")
			}
		}
		fmt.Println("Updating role ...")
		roleId = utl.Str(existing["name"])
	}

	payload := x                                             // Obviously using x object as the payload
	params := map[string]string{"api-version": "2022-04-01"} // roleDefinitions
	url := ConstAzUrl + xScope1 + "/providers/Microsoft.Authorization/roleDefinitions/" + roleId
	r, statusCode, _ := ApiPut(url, z, payload, params)
	if statusCode == 201 {
		PrintRoleDefinition(r, z) // Print the newly updated object
	} else {
		e := r["error"].(map[string]interface{})
		fmt.Println(e["message"].(string))
	}
}

// Deletes an RBAC role definition object by its fully qualified object Id
// Example of a fully qualified Id string:
//
//	"/providers/Microsoft.Authorization/roleDefinitions/50a6ff7c-3ac5-4acc-b4f4-9a43aee0c80f"
func DeleteAzRoleDefinitionByFqid(fqid string, z Bundle) map[string]interface{} {
	params := map[string]string{"api-version": "2022-04-01"} // roleDefinitions
	url := ConstAzUrl + fqid
	r, statusCode, _ := ApiDelete(url, z, params)
	//ApiErrorCheck("DELETE", url, utl.Trace(), r)
	if statusCode != 200 {
		if statusCode == 204 {
			fmt.Println("Role definition already deleted or does not exist. Give Azure a minute to flush it out.")
		} else {
			e := r["error"].(map[string]interface{})
			fmt.Println(e["message"].(string))
		}
	}
	return nil
}

// Returns id:name map of all RBAC role definitions
func GetIdMapRoleDefs(z Bundle) (nameMap map[string]string) {
	nameMap = make(map[string]string)
	roleDefs := GetMatchingRoleDefinitions("", false, z) // false = don't force going to Azure
	// By not forcing an Azure call we're opting for cache speed over id:name map accuracy
	for _, i := range roleDefs {
		x := i.(map[string]interface{})
		if x["name"] != nil {
			xProp := x["properties"].(map[string]interface{})
			if xProp["roleName"] != nil {
				nameMap[utl.Str(x["name"])] = utl.Str(xProp["roleName"])
			}
		}
	}
	return nameMap
}

// Dedicated role definition local cache counter able to discern if role is custom to native tenant or it's an Azure BuilIn role
func RoleDefinitionCountLocal(z Bundle) (builtin, custom int64) {
	var customList []interface{} = nil
	var builtinList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_roleDefinitions."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			definitions := rawList.([]interface{})
			for _, i := range definitions {
				x := i.(map[string]interface{}) // Assert as JSON object type
				xProp := x["properties"].(map[string]interface{})
				if utl.Str(xProp["type"]) == "CustomRole" {
					customList = append(customList, x)
				} else {
					builtinList = append(builtinList, x)
				}
			}
			return int64(len(builtinList)), int64(len(customList))
		}
	}
	return 0, 0
}

// Counts all role definition in Azure. Returns 2 lists: one of native custom roles, the other of built-in role
func RoleDefinitionCountAzure(z Bundle) (builtin, custom int64) {
	var customList []interface{} = nil
	var builtinList []interface{} = nil
	definitions := GetAzRoleDefinitions(z, false) // false = be silent
	for _, i := range definitions {
		x := i.(map[string]interface{}) // Assert as JSON object type
		xProp := x["properties"].(map[string]interface{})
		if utl.Str(xProp["type"]) == "CustomRole" {
			customList = append(customList, x)
		} else {
			builtinList = append(builtinList, x)
		}
	}
	return int64(len(builtinList)), int64(len(customList))
}

// Gets all role definitions matching on 'filter'. Returns entire list if filter is empty ""
func GetMatchingRoleDefinitions(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_roleDefinitions."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstAzCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstAzCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzRoleDefinitions(z, true)
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
		// Match against relevant strings within roleDefinitions JSON object (Note: Not all attributes are maintained)
		if utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
		}
	}
	return matchingList
}

// Gets all role definitions in current Azure tenant and save them to local cache file
// Option to be verbose (true) or quiet (false), since it can take a while.
// References:
//
//	https://learn.microsoft.com/en-us/azure/role-based-access-control/role-definitions-list
//	https://learn.microsoft.com/en-us/rest/api/authorization/role-definitions/list
func GetAzRoleDefinitions(z Bundle, verbose bool) (list []interface{}) {
	list = nil             // We have to zero it out
	var uniqueIds []string // Keep track of assignment objects
	k := 1                 // Track number of API calls to provide progress

	var mgGroupNameMap, subNameMap map[string]string
	if verbose {
		mgGroupNameMap = GetIdMapMgGroups(z)
		subNameMap = GetIdMapSubs(z)
	}

	scopes := GetAzRbacScopes(z)                             // Get all scopes
	params := map[string]string{"api-version": "2022-04-01"} // roleDefinitions
	for _, scope := range scopes {
		url := ConstAzUrl + scope + "/providers/Microsoft.Authorization/roleDefinitions"
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
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_roleDefinitions."+ConstCacheFileExtension)
	utl.SaveFileJsonGzip(list, cacheFile) // Update the local cache
	return list
}

// Gets role definition by displayName
// See https://learn.microsoft.com/en-us/rest/api/authorization/role-definitions/list
func GetAzRoleDefinitionByName(roleName string, z Bundle) (y map[string]interface{}) {
	y = nil
	scopes := GetAzRbacScopes(z) // Get all scopes
	params := map[string]string{
		"api-version": "2022-04-01", // roleDefinitions
		"$filter":     "roleName eq '" + roleName + "'",
	}
	for _, scope := range scopes {
		url := ConstAzUrl + scope + "/providers/Microsoft.Authorization/roleDefinitions"
		r, _, _ := ApiGet(url, z, params)
		ApiErrorCheck("GET", url, utl.Trace(), r) // DEBUG. Until ApiGet rewrite with nullable _ err
		if r != nil && r["value"] != nil {
			results := r["value"].([]interface{})
			if len(results) == 1 {
				y = results[0].(map[string]interface{}) // Select first, only index entry
				return y                                // We found it
			}
		}
	}
	// If above logic ever finds than 1, then we have serious issuses, just nil below
	return nil
}

// Gets role definition object if it exists exactly as x object (as per essential attributes).
// Matches on: displayName and assignableScopes
func GetAzRoleDefinitionByObject(x map[string]interface{}, z Bundle) (y map[string]interface{}) {
	// First, make sure x is a searchable role definition object
	if x == nil { // Don't look for empty objects
		return nil
	}
	xProp := x["properties"].(map[string]interface{})
	if xProp == nil {
		return nil
	}

	xScopes := xProp["assignableScopes"].([]interface{})
	if utl.GetType(xScopes)[0] != '[' || len(xScopes) < 1 {
		return nil // Return nil if assignableScopes not an array, or it's empty
	}
	xRoleName := utl.Str(xProp["roleName"])
	if xRoleName == "" {
		return nil
	}

	// Look for x under all its scopes
	for _, i := range xScopes {
		scope := utl.Str(i)
		if scope == "/" {
			scope = ""
		} // Highly unlikely but just to avoid an err
		// Get all role assignments for xPrincipalId under xScope
		params := map[string]string{
			"api-version": "2022-04-01", // roleDefinitions
			"$filter":     "roleName eq '" + xRoleName + "'",
		}
		url := ConstAzUrl + scope + "/providers/Microsoft.Authorization/roleDefinitions"
		r, _, _ := ApiGet(url, z, params)
		ApiErrorCheck("GET", url, utl.Trace(), r)
		if r != nil && r["value"] != nil {
			results := r["value"].([]interface{})
			if len(results) == 1 {
				y = results[0].(map[string]interface{}) // Select first index entry
				return y                                // We found it
			} else {
				return nil // If there's more than one entry we have other problems, so just return nil
			}
		}
	}
	return nil
}

// Gets role definition by Object Id. Unfortunately we have to iterate
// through the entire tenant scope hierarchy, which can take time.
func GetAzRoleDefinitionByUuid(uuid string, z Bundle) map[string]interface{} {
	scopes := GetAzRbacScopes(z)
	params := map[string]string{"api-version": "2022-04-01"} // roleDefinitions
	for _, scope := range scopes {
		url := ConstAzUrl + scope + "/providers/Microsoft.Authorization/roleDefinitions/" + uuid
		r, _, _ := ApiGet(url, z, params)
		if r != nil && r["id"] != nil {
			return r // Return as soon as we find a match
		}
	}
	return nil
}
