package maz

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/queone/utl"
)

// Creates or updates a role definition or assignment based on given specfile
func UpsertAzObject(force bool, filePath string, z Bundle) {
	if utl.FileNotExist(filePath) || utl.FileSize(filePath) < 1 {
		utl.Die("File does not exist, or it is zero size\n")
	}
	formatType, t, x := GetObjectFromFile(filePath)
	if formatType != "JSON" && formatType != "YAML" {
		utl.Die("File is not in JSON nor YAML format\n")
	}
	if t != "d" && t != "a" {
		utl.Die("File is not a role definition nor an assignment specfile\n")
	}
	switch t {
	case "d":
		UpsertAzRoleDefinition(force, x, z)
	case "a":
		CreateAzRoleAssignment(x, z)
	}
	os.Exit(0)
}

// Deletes object based on string specifier (currently only supports roleDefinitions or Assignments)
// String specifier can be either of 3: UUID, specfile, or displaName (only for roleDefinition)
// 1) Search Azure by given identifier; 2) Grab object's Fully Qualified Id string;
// 3) Print and prompt for confirmation; 4) Delete or abort
func DeleteAzObject(force bool, specifier string, z Bundle) {
	if utl.ValidUuid(specifier) {
		list := FindAzObjectsByUuid(specifier, z) // Get all objects that may match this UUID, hopefully just one
		if len(list) > 1 {
			utl.Die(utl.Red("UUID collision? Run utility with UUID argument to see the list.\n"))
		}
		if len(list) < 1 {
			utl.Die("Object does not exist.\n")
		}
		y := list[0].(map[string]interface{}) // Single out the only object
		if y != nil && y["mazType"] != nil {
			t := utl.Str(y["mazType"])
			fqid := utl.Str(y["id"]) // Grab fully qualified object Id
			PrintObject(t, y, z)
			if !force {
				if utl.PromptMsg("DELETE above? y/n ") != 'y' {
					utl.Die("Aborted.\n")
				}
			}
			switch t {
			case "d":
				DeleteAzRoleDefinitionByFqid(fqid, z)
			case "a":
				DeleteAzRoleAssignmentByFqid(fqid, z)
			}
		}
	} else if utl.FileExist(specifier) {
		// Delete object defined in specfile
		formatType, t, x := GetObjectFromFile(specifier) // x is for the object in Specfile
		if formatType != "JSON" && formatType != "YAML" {
			utl.Die("File is not in JSON nor YAML format\n")
		}
		var y map[string]interface{} = nil
		switch t {
		case "d":
			y = GetAzRoleDefinitionByObject(x, z) // y is for the object from Azure
			fqid := utl.Str(y["id"])              // Grab fully qualified object Id
			if y == nil {
				utl.Die("Role definition does not exist.\n")
			} else {
				PrintRoleDefinition(y, z) // Use specific role def print function
				if !force {
					if utl.PromptMsg("DELETE above? y/n ") != 'y' {
						utl.Die("Aborted.\n")
					}
				}
				DeleteAzRoleDefinitionByFqid(fqid, z)
			}
		case "a":
			y = GetAzRoleAssignmentByObject(x, z)
			fqid := utl.Str(y["id"]) // Grab fully qualified object Id
			if y == nil {
				utl.Die("Role assignment does not exist.\n")
			} else {
				PrintRoleAssignment(y, z) // Use specific role assgmnt print function
				if !force {
					if utl.PromptMsg("DELETE above? y/n ") != 'y' {
						utl.Die("Aborted.\n")
					}
				}
				DeleteAzRoleAssignmentByFqid(fqid, z)
			}
		default:
			utl.Die("File " + formatType + " is not a role definition or assignment.\n")
		}
	} else {
		// Delete role definition by its displayName, if it exists. This only applies to definitions
		// since assignments do not have a displayName attribute. Also, other objects are not supported.
		y := GetAzRoleDefinitionByName(specifier, z)
		if y == nil {
			utl.Die("Role definition does not exist.\n")
		}
		fqid := utl.Str(y["id"]) // Grab fully qualified object Id
		PrintRoleDefinition(y, z)
		if !force {
			if utl.PromptMsg("DELETE above? y/n ") != 'y' {
				utl.Die("Aborted.\n")
			}
		}
		DeleteAzRoleDefinitionByFqid(fqid, z)
	}
}

// Returns list of Azure objects with this UUID. We are saying a list because 1)
// the UUID could be an appId shared by an app and an SP, or 2) there could be
// UUID collisions with multiple objects potentially sharing the same UUID. Only
// checks for the maz package limited set of Azure object types.
func FindAzObjectsByUuid(uuid string, z Bundle) (list []interface{}) {
	list = nil
	for _, t := range mazTypes {
		x := GetAzObjectByUuid(t, uuid, z)
		if x != nil && x["id"] != nil { // Valid objects have an 'id' attribute
			// Found one of these types with this UUID
			x["mazType"] = t // Extend object with mazType as an ADDITIONAL field
			list = append(list, x)
		}
	}
	return list
}

// Retrieves Azure object by Object UUID
func GetAzObjectByUuid(t, uuid string, z Bundle) (x map[string]interface{}) {
	switch t {
	case "d":
		return GetAzRoleDefinitionByUuid(uuid, z)
	case "a":
		return GetAzRoleAssignmentByUuid(uuid, z)
	case "s":
		return GetAzSubscriptionByUuid(uuid, z)
	case "u":
		return GetAzUserByUuid(uuid, z)
	case "g":
		return GetAzGroupByUuid(uuid, z)
	case "sp":
		return GetAzSpByUuid(uuid, z)
	case "ap":
		return GetAzAppByUuid(uuid, z)
	case "ad":
		return GetAzAdRoleByUuid(uuid, z)
	}
	return nil
}

// Gets all scopes in the Azure tenant RBAC hierarchy: Tenant Root Group and all
// management groups, plus all subscription scopes
func GetAzRbacScopes(z Bundle) (scopes []string) {
	scopes = nil
	managementGroups := GetAzMgGroups(z) // Start by adding all the managementGroups scopes
	for _, i := range managementGroups {
		x := i.(map[string]interface{})
		scopes = append(scopes, utl.Str(x["id"]))
	}
	subIds := GetAzSubscriptionsIds(z) // Now add all the subscription scopes
	scopes = append(scopes, subIds...)

	// SCOPES below subscriptions do not appear to be REALLY NEEDED. Most list
	// search functions pull all objects in lower scopes. If there is a future
	// need to keep drilling down, next level being Resource Group scopes, then
	// they can be acquired with something like below:

	// params := map[string]string{"api-version": "2021-04-01"} // resourceGroups
	// for subId := range subIds {
	// 	url := ConstAzUrl + subId + "/resourcegroups"
	// 	r, _, _ := ApiGet(url, z, params)
	// 	if r != nil && r["value"] != nil {
	// 		resourceGroups := r["value"].([]interface{})
	// 		for _, j := range resourceGroups {
	// 			y := j.(map[string]interface{})
	// 			rgId := utl.Str(y["id"])
	// 			scopes = append(scopes, rgId)
	// 		}
	// 	}
	// }
	// // Then repeat for next leval scope ...

	return scopes
}

// Retrieves locally cached list of objects in given cache file
func GetCachedObjects(cacheFile string) (cachedList []interface{}) {
	cachedList = nil
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList = rawList.([]interface{})
		}
	}
	return cachedList
}

// Generic function to get objects of type t whose attributes match on filter.
// If filter is the "" empty string return ALL of the objects of this type.
func GetObjects(t, filter string, force bool, z Bundle) (list []interface{}) {
	switch t {
	case "d":
		return GetMatchingRoleDefinitions(filter, force, z)
	case "a":
		return GetMatchingRoleAssignments(filter, force, z)
	case "m":
		return GetMatchingMgGroups(filter, force, z)
	case "s":
		return GetMatchingSubscriptions(filter, force, z)
	case "ap":
		return GetMatchingApps(filter, force, z)
	case "g":
		return GetMatchingGroups(filter, force, z)
	case "ad":
		return GetMatchingAdRoles(filter, force, z)
	case "sp":
		return GetMatchingSps(filter, force, z)
	case "u":
		return GetMatchingUsers(filter, force, z)
	}
	return nil
}

// Returns all Azure pages for given API URL call
func GetAzAllPages(url string, z Bundle) (list []interface{}) {
	list = nil
	r, _, _ := ApiGet(url, z, nil)
	for {
		// Forver loop until there are no more pages
		var thisBatch []interface{} = nil // Assume zero entries in this batch
		if r["value"] != nil {
			thisBatch = r["value"].([]interface{})
			if len(thisBatch) > 0 {
				list = append(list, thisBatch...) // Continue growing list
			}
		}
		nextLink := utl.Str(r["@odata.nextLink"])
		if nextLink == "" {
			break // Break once there is no more pages
		}
		r, _, _ = ApiGet(nextLink, z, nil) // Get next batch
	}
	return list
}

// Generic Azure object deltaSet retriever function. Returns the set of changed or new items,
// and a deltaLink for running the next future Azure query. Implements the pattern described at
// https://docs.microsoft.com/en-us/graph/delta-query-overview
func GetAzObjects(url string, z Bundle, verbose bool) (deltaSet []interface{}, deltaLinkMap map[string]interface{}) {
	k := 1 // Track number of API calls
	r, _, _ := ApiGet(url, z, nil)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	for {
		// Infinite for-loop until deltaLink appears (meaning we're done getting current delta set)
		var thisBatch []interface{} = nil // Assume zero entries in this batch
		var objCount int = 0
		if r["value"] != nil {
			thisBatch = r["value"].([]interface{})
			objCount = len(thisBatch)
			if objCount > 0 {
				deltaSet = append(deltaSet, thisBatch...) // Continue growing deltaSet
			}
		}
		if verbose {
			// Progress count indicator. Using global var rUp to overwrite last line. Defer newline until done
			fmt.Printf("%sAPI call %d: %d objects", rUp, k, objCount)
		}
		if r["@odata.deltaLink"] != nil {
			deltaLinkMap := map[string]interface{}{"@odata.deltaLink": utl.Str(r["@odata.deltaLink"])}
			if verbose {
				fmt.Printf("\n")
			}
			return deltaSet, deltaLinkMap // Return immediately after deltaLink appears
		}
		r, _, _ = ApiGet(utl.Str(r["@odata.nextLink"]), z, nil) // Get next batch
		//ApiErrorCheck("GET", url, utl.Trace(), r)
		k++
	}
}

// Removes specified cache file
func RemoveCacheFile(t string, z Bundle) {
	switch t {
	case "id":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.CredsFile))
	case "t":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TokenFile))
	case "d":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_roleDefinitions."+ConstCacheFileExtension))
	case "a":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_roleAssignments."+ConstCacheFileExtension))
	case "s":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_subscriptions."+ConstCacheFileExtension))
	case "m":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_managementGroups."+ConstCacheFileExtension))
	case "u":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_users."+ConstCacheFileExtension))
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_users_deltaLink."+ConstCacheFileExtension))
	case "g":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_groups."+ConstCacheFileExtension))
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_groups_deltaLink."+ConstCacheFileExtension))
	case "sp":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_servicePrincipals."+ConstCacheFileExtension))
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_servicePrincipals_deltaLink."+ConstCacheFileExtension))
	case "ap":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_applications."+ConstCacheFileExtension))
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_applications_deltaLink."+ConstCacheFileExtension))
	case "ad":
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_directoryRoles."+ConstCacheFileExtension))
		utl.RemoveFile(filepath.Join(z.ConfDir, z.TenantId+"_directoryRoles_deltaLink."+ConstCacheFileExtension))
	case "all":
		// See https://stackoverflow.com/questions/48072236/remove-files-with-wildcard
		fileList, err := filepath.Glob(filepath.Join(z.ConfDir, z.TenantId+"_*."+ConstCacheFileExtension))
		if err != nil {
			panic(err)
		}
		for _, filePath := range fileList {
			utl.RemoveFile(filePath)
		}
	}
}

// Returns 3 values: File format type, single-letter object type, and the object itself
func GetObjectFromFile(filePath string) (formatType, t string, obj map[string]interface{}) {
	// Because JSON is essentially a subset of YAML, we have to check JSON first
	// As an interesting aside regarding YAML & JSON, see https://news.ycombinator.com/item?id=31406473
	formatType = "JSON"                         // Pretend it's JSON
	objRaw, _ := utl.LoadFileJsonGzip(filePath) // Ignores the errors
	if objRaw == nil {                          // Ok, it's NOT JSON
		objRaw, _ = utl.LoadFileYaml(filePath) // See if it's YAML, ignoring the error
		if objRaw == nil {
			return "", "", nil // Ok, it's neither, let's return 3 null values
		}
		formatType = "YAML" // It is YAML
	}
	obj = objRaw.(map[string]interface{})

	// Continue unpacking the object to see what it is
	xProp, err := obj["properties"].(map[string]interface{})
	if !err { // Valid definition/assignments have a properties attribute
		return formatType, "", nil // It's not a valid object, return null for type and object
	}
	roleName := utl.Str(xProp["roleName"])       // Assert and assume it's a definition
	roleId := utl.Str(xProp["roleDefinitionId"]) // assert and assume it's an assignment

	if roleName != "" {
		return formatType, "d", obj // Role definition
	} else if roleId != "" {
		return formatType, "a", obj // Role assignment
	} else {
		return formatType, "", obj // Unknown
	}
}

// Compares specification file to what is in Azure
func CompareSpecfileToAzure(filePath string, z Bundle) {
	if utl.FileNotExist(filePath) || utl.FileSize(filePath) < 1 {
		utl.Die("File does not exist, or is zero size\n")
	}
	formatType, t, fileDef := GetObjectFromFile(filePath)
	if (formatType != "JSON" && formatType != "YAML" && t != "d" && t != "a") || t == "" {
		utl.Die("File is not a properly defined role definition or assignment.\n")
	}

	if t == "d" {
		azureDef := GetAzRoleDefinitionByObject(fileDef, z)
		if azureDef == nil {
			fileProp := fileDef["properties"].(map[string]interface{})
			fileRoleName := utl.Str(fileProp["roleName"])
			fmt.Printf("Role " + utl.Mag(fileRoleName) + " as defined in specfile does " + utl.Red("not") + " exist in Azure.\n")
		} else {
			fmt.Printf("Role definition in specfile " + utl.Gre("already") + " exist in Azure. See details below:\n")
			DiffRoleDefinitionSpecfileVsAzure(fileDef, azureDef, z)
		}
	} else {
		azureDef := GetAzRoleAssignmentByObject(fileDef, z)
		if azureDef == nil {
			fmt.Printf("Role assignment in specfile does " + utl.Red("not") + " exist in Azure.\n")
		} else {
			fmt.Printf("Role assignment in specfile " + utl.Gre("already") + " exist in Azure. See details below:\n")
			PrintRoleAssignment(azureDef, z)
		}
	}
	os.Exit(0)
}
