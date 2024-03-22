package maz

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/queone/utl"
)

// Prints service principal object in YAML-like format
func PrintSp(x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}
	id := utl.Str(x["id"])

	// Print the most important attributes
	list := []string{"id", "displayName", "appId"}
	for _, i := range list {
		v := utl.Str(x[i])
		if v != "" { // Only print non-null attributes
			fmt.Printf("%s: %s\n", utl.Blu(i), utl.Gre(v))
		}
	}

	// Print certificates keys
	url := ConstMgUrl + "/v1.0/servicePrincipals/" + id + "/keyCredentials"
	r, statusCode, _ := ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil && len(r["value"].([]interface{})) > 0 {
		keyCredentials := r["value"].([]interface{}) // Assert as JSON array
		if keyCredentials != nil {
			PrintCertificateList(keyCredentials)
		}
	}

	// Print secret expiry and other details. Not actual secretText, which cannot be retrieve anyway!
	url = ConstMgUrl + "/v1.0/servicePrincipals/" + id + "/passwordCredentials"
	r, statusCode, _ = ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil && len(r["value"].([]interface{})) > 0 {
		passwordCredentials := r["value"].([]interface{}) // Assert as JSON array
		if passwordCredentials != nil {
			PrintSecretList(passwordCredentials)
		}
	}

	// Print owners
	url = ConstMgUrl + "/beta/servicePrincipals/" + id + "/owners"
	r, statusCode, _ = ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil {
		PrintOwners(r["value"].([]interface{}))
	}

	// This part does 2 things:
	// 1) creates the role:name map to used later when calling PrintAppRoleAssignments()
	// 2) prints all app_roles
	roleNameMap := make(map[string]string)
	roleNameMap["00000000-0000-0000-0000-000000000000"] = "Default" // Include default app permissions role
	appRoles := x["appRoles"].([]interface{})
	if len(appRoles) > 0 {
		fmt.Printf(utl.Blu("app_roles") + ":\n")
		for _, i := range appRoles {
			a := i.(map[string]interface{})
			rId := utl.Str(a["id"])
			displayName := utl.Str(a["displayName"])
			roleNameMap[rId] = displayName // Update growing list of roleNameMap
			if len(displayName) >= 60 {
				displayName = utl.FirstN(displayName, 57) + "..."
			}
			fmt.Printf("  %s  %-50s  %-60s\n", utl.Gre(rId), utl.Gre(utl.Str(a["value"])), utl.Gre(displayName))
		}
	}

	// Print app role assignment members and the specific role assigned
	url = ConstMgUrl + "/beta/servicePrincipals/" + id + "/appRoleAssignedTo"
	appRoleAssignments := GetAzAllPages(url, z)
	PrintAppRoleAssignmentsSp(roleNameMap, appRoleAssignments) // roleNameMap is used here

	// Print all groups and roles it is a member of
	url = ConstMgUrl + "/beta/servicePrincipals/" + id + "/transitiveMemberOf"
	r, statusCode, _ = ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil {
		memberOf := r["value"].([]interface{})
		PrintMemberOfs("g", memberOf)
	}

	// Print API permissions
	// - https://learn.microsoft.com/en-us/entra/identity-platform/app-objects-and-service-principals?tabs=browser
	// - https://learn.microsoft.com/en-us/entra/identity-platform/permissions-consent-overview
	var apiPerms [][]string = nil
	// First, lets gather the delegated permissions
	url = ConstMgUrl + "/v1.0/servicePrincipals/" + id + "/oauth2PermissionGrants"
	r, statusCode, _ = ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil && len(r["value"].([]interface{})) > 0 {
		oauth2Perms := r["value"].([]interface{}) // Assert as JSON array
		// Collate each OAuth 2.0 scope
		for _, i := range oauth2Perms {
			api := i.(map[string]interface{}) // Assert as JSON object
			// utl.PrintJsonColor(api) // DEBUG

			apiId := utl.Str(api["id"])              // This api assignment ID is used to delete it if ever necessary
			resourceId := utl.Str(api["resourceId"]) // Get API's SP to get its displayName
			url2 := ConstMgUrl + "/v1.0/servicePrincipals/" + resourceId
			r2, _, _ := ApiGet(url2, z, nil)
			apiName := "Unknown"
			if r2["displayName"] != nil {
				apiName = utl.Str(r2["displayName"])
			}
			// Collect each delegated claim for this perm
			scope := strings.TrimSpace(utl.Str(api["scope"]))
			claims := strings.Split(scope, " ")
			for _, j := range claims {
				apiPerms = append(apiPerms, []string{apiId, apiName, "Delegated", j})
			}
		}
	}
	// Secondly, lets gather the application permissions
	url = ConstMgUrl + "/v1.0/servicePrincipals/" + id + "/appRoleAssignments"
	r, statusCode, _ = ApiGet(url, z, nil)
	uniqueResIds := make(map[string]struct{}) // Unique resourceIds (SPs)
	if statusCode == 200 && r != nil && r["value"] != nil && len(r["value"].([]interface{})) > 0 {
		apiAssignments := r["value"].([]interface{}) // Assert as JSON array
		// Collate assignments for each API
		for _, i := range apiAssignments {
			api := i.(map[string]interface{}) // Assert as JSON object
			//utl.PrintJsonColor(api) // DEBUG

			apiId := utl.Str(api["id"]) // This api assignment ID is used to delete it if ever necessary
			apiName := utl.Str(api["resourceDisplayName"])
			resourceId := utl.Str(api["resourceId"])
			appRoleId := utl.Str(api["appRoleId"])
			j := resourceId + "/" + appRoleId

			// Keeping track of unique resourceIds speeds up and simplifies getting permission value names below
			uniqueResIds[resourceId] = struct{}{} // Go mem optimization trick, since we only care about the key

			apiPerms = append(apiPerms, []string{apiId, apiName, "Application", j})
		}
	}
	// Create the resId/roleId:value map
	roleMap := make(map[string]string)
	for resId := range uniqueResIds {
		url := ConstMgUrl + "/beta/servicePrincipals/" + resId
		r, _, _ := ApiGet(url, z, nil)
		if r["appRoles"] != nil {
			for _, i := range r["appRoles"].([]interface{}) {
				role := i.(map[string]interface{})
				k := resId + "/" + utl.Str(role["id"])
				roleMap[k] = utl.Str(role["value"])
			}
		}
	}
	// Now print them
	if len(apiPerms) > 0 {
		fmt.Printf(utl.Blu("oauth2PermissionGrants") + ":\n")
		for _, v := range apiPerms {
			perm := v[3]
			if utl.ValidUuid(strings.Split(v[3], "/")[0]) {
				perm = roleMap[v[2]]
			}
			// // TODO: Sort by the 3rd column
			// import "sort"
			// sort.Slice(myList, func(i, j int) bool {
			// 	return myList[i][2] < myList[j][2]
			// })
			// API Name | Permission | Type
			fmt.Printf("  %s%s  %s%s  %s%s  %s\n", utl.Gre(v[0]), utl.PadSpaces(40, len(v[0])),
				utl.Gre(v[1]), utl.PadSpaces(40, len(v[1])),
				utl.Gre(v[2]), utl.PadSpaces(14, len(v[2])), utl.Gre(perm))
		}
	}
}

// Creates/adds a secret to the given SP
func AddSpSecret(uuid, displayName, expiry string, z Bundle) {
	if !utl.ValidUuid(uuid) {
		utl.Die("Invalid SP UUID.\n")
	}
	var endDateTime string
	if utl.ValidDate(expiry, "2006-01-02") {
		var err error
		endDateTime, err = utl.ConvertDateFormat(expiry, "2006-01-02", time.RFC3339Nano)
		if err != nil {
			utl.Die("Error converting Expiry date format to RFC3339Nano/ISO8601 format.\n")
		}
	} else {
		// If expiry not a valid date, see if it's a valid integer number
		days, err := utl.StringToInt64(expiry)
		if err != nil {
			utl.Die("Error converting Expiry to valid integer number.\n")
		}
		maxDays := utl.GetDaysSinceOrTo("9999-12-31") // Maximum supported date
		if days > maxDays {
			days = maxDays
		}
		expiryTime := utl.GetDateInDays(utl.Int64ToString(days)) // Set expiryTime to 'days' from now
		expiry = expiryTime.Format("2006-01-02")                 // Convert it to yyyy-mm-dd format
		endDateTime = expiryTime.Format(time.RFC3339Nano)        // Convert to RFC3339Nano/ISO8601 format
	}

	payload := map[string]interface{}{
		"passwordCredential": map[string]string{
			"displayName": displayName,
			"endDateTime": endDateTime,
		},
	}
	url := ConstMgUrl + "/v1.0/servicePrincipals/" + uuid + "/addPassword"
	r, statusCode, _ := ApiPost(url, z, payload, nil)
	if statusCode == 200 {
		fmt.Printf("%s: %s\n", utl.Blu("App_Object_Id"), utl.Gre(uuid))
		fmt.Printf("%s: %s\n", utl.Blu("New_Secret_Id"), utl.Gre(utl.Str(r["keyId"])))
		fmt.Printf("%s: %s\n", utl.Blu("New_Secret_Name"), utl.Gre(displayName))
		fmt.Printf("%s: %s\n", utl.Blu("New_Secret_Expiry"), utl.Gre(expiry))
		fmt.Printf("%s: %s\n", utl.Blu("New_Secret_Text"), utl.Gre(utl.Str(r["secretText"])))
	} else {
		e := r["error"].(map[string]interface{})
		utl.Die(e["message"].(string) + "\n")
	}
}

// Removes a secret from the given SP
func RemoveSpSecret(uuid, keyId string, z Bundle) {
	if !utl.ValidUuid(uuid) {
		utl.Die("SP UUID is not a valid UUID.\n")
	}
	if !utl.ValidUuid(keyId) {
		utl.Die("Secret ID is not a valid UUID.\n")
	}

	// Get SP, display details and secret, and prompt for delete confirmation
	x := GetAzSpByUuid(uuid, z)
	if x == nil || x["id"] == nil {
		utl.Die("There's no SP with this UUID.\n")
	}
	url := ConstMgUrl + "/v1.0/servicePrincipals/" + uuid + "/passwordCredentials"
	r, statusCode, _ := ApiGet(url, z, nil)
	var passwordCredentials []interface{} = nil
	if statusCode == 200 && r != nil && r["value"] != nil && len(r["value"].([]interface{})) > 0 {
		passwordCredentials = r["value"].([]interface{}) // Assert as JSON array
	}
	if passwordCredentials == nil || len(passwordCredentials) < 1 {
		utl.Die("SP object has no secrets.\n")
	}
	var a map[string]interface{} = nil // Target keyId, Secret ID to be deleted
	for _, i := range passwordCredentials {
		targetKeyId := i.(map[string]interface{})
		if utl.Str(targetKeyId["keyId"]) == keyId {
			a = targetKeyId
			break
		}
	}
	if a == nil {
		utl.Die("SP object does not have this Secret ID.\n")
	}
	cId := utl.Str(a["keyId"])
	cName := utl.Str(a["displayName"])
	cHint := utl.Str(a["hint"]) + "********"
	cStart, err := utl.ConvertDateFormat(utl.Str(a["startDateTime"]), time.RFC3339Nano, "2006-01-02")
	if err != nil {
		utl.Die(utl.Trace() + err.Error() + "\n")
	}
	cExpiry, err := utl.ConvertDateFormat(utl.Str(a["endDateTime"]), time.RFC3339Nano, "2006-01-02")
	if err != nil {
		utl.Die(utl.Trace() + err.Error() + "\n")
	}

	// Prompt
	fmt.Printf("%s: %s\n", utl.Blu("id"), utl.Gre(utl.Str(x["id"])))
	fmt.Printf("%s: %s\n", utl.Blu("appId"), utl.Gre(utl.Str(x["appId"])))
	fmt.Printf("%s: %s\n", utl.Blu("displayName"), utl.Gre(utl.Str(x["displayName"])))
	fmt.Printf("%s:\n", utl.Yel("secret_to_be_deleted"))
	fmt.Printf("  %-36s  %-30s  %-16s  %-16s  %s\n", utl.Yel(cId), utl.Yel(cName),
		utl.Yel(cHint), utl.Yel(cStart), utl.Yel(cExpiry))
	if utl.PromptMsg(utl.Yel("DELETE above? y/n ")) == 'y' {
		payload := map[string]interface{}{"keyId": keyId}
		url := ConstMgUrl + "/v1.0/servicePrincipals/" + uuid + "/removePassword"
		r, statusCode, _ := ApiPost(url, z, payload, nil)
		if statusCode == 204 {
			utl.Die("Successfully deleted secret.\n")
		} else {
			e := r["error"].(map[string]interface{})
			utl.Die(e["message"].(string) + "\n")
		}
	} else {
		utl.Die("Aborted.\n")
	}
}

// Retrieves counts of all SPs in local cache, 2 values: Native ones to this tenant, and all others
func SpsCountLocal(z Bundle) (native, microsoft int64) {
	var nativeList []interface{} = nil
	var microsoftList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_servicePrincipals."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList := rawList.([]interface{})
			for _, i := range cachedList {
				x := i.(map[string]interface{})
				if utl.Str(x["appOwnerOrganizationId"]) == z.TenantId { // If owned by current tenant ...
					nativeList = append(nativeList, x)
				} else {
					microsoftList = append(microsoftList, x)
				}
			}
			return int64(len(nativeList)), int64(len(microsoftList))
		}
	}
	return 0, 0
}

// Retrieves counts of all SPs in this Azure tenant, 2 values: Native ones to this tenant, and all others
func SpsCountAzure(z Bundle) (native, microsoft int64) {
	// First, get total number of SPs in tenant
	var all int64 = 0
	z.MgHeaders["ConsistencyLevel"] = "eventual"
	//baseUrl := ConstMgUrl + "/v1.0/servicePrincipals"
	baseUrl := ConstMgUrl + "/beta/servicePrincipals"
	url := baseUrl + "/$count"
	r, _, _ := ApiGet(url, z, nil)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	if r["value"] == nil {
		return 0, 0 // Something went wrong, so return zero for both
	}
	all = r["value"].(int64)

	// Now get count of SPs registered and native to only this tenant
	params := map[string]string{"$filter": "appOwnerOrganizationId eq " + z.TenantId}
	params["$count"] = "true"
	url = baseUrl
	r, _, _ = ApiGet(url, z, params)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	if r["value"] == nil {
		return 0, all // Something went wrong with native count, retun all as Microsoft ones
	}

	native = int64(r["@odata.count"].(float64))
	microsoft = all - native

	return native, microsoft
}

// Returns an id:name map of all service principals
func GetIdMapSps(z Bundle) (nameMap map[string]string) {
	nameMap = make(map[string]string)
	sps := GetMatchingSps("", false, z) // false = don't force a call to Azure
	// By not forcing an Azure call we're opting for cache speed over id:name map accuracy
	for _, i := range sps {
		x := i.(map[string]interface{})
		if x["id"] != nil && x["displayName"] != nil {
			nameMap[utl.Str(x["id"])] = utl.Str(x["displayName"])
		}
	}
	return nameMap
}

// Gets all service principals matching on 'filter'. Return entire list if filter is empty ""
func GetMatchingSps(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_servicePrincipals."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstMgCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstMgCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzSps(z, true)
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
		// Match against relevant strings within SP JSON object (Note: Not all attributes are maintained)
		if !utl.ItemInList(id, ids) && utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
			ids = append(ids, id)
		}
	}
	return matchingList
}

// Gets all service principals from Azure and sync to local cache. Shows progress if verbose = true
func GetAzSps(z Bundle, verbose bool) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_servicePrincipals."+ConstCacheFileExtension)
	deltaLinkFile := filepath.Join(z.ConfDir, z.TenantId+"_servicePrincipals_deltaLink."+ConstCacheFileExtension)

	baseUrl := ConstMgUrl + "/beta/servicePrincipals"
	// Get delta updates only if/when below attributes in $select are modified
	selection := "?$select=displayName,appId,accountEnabled,appOwnerOrganizationId,passwordCredentials"
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

// Gets service principal by its Object UUID or by its appId, with all attributes
func GetAzSpByUuid(uuid string, z Bundle) map[string]interface{} {
	baseUrl := ConstMgUrl + "/beta/servicePrincipals"
	selection := "?$select=*"
	url := baseUrl + "/" + uuid + selection // First search is for direct Object Id
	r, _, _ := ApiGet(url, z, nil)
	if r != nil && r["error"] != nil {
		// Second search is for this SP's application Client Id
		url = baseUrl + selection
		params := map[string]string{"$filter": "appId eq '" + uuid + "'"}
		r, _, _ := ApiGet(url, z, params)
		if r != nil && r["value"] != nil {
			list := r["value"].([]interface{})
			count := len(list)
			if count == 1 {
				return list[0].(map[string]interface{}) // Return single value found
			} else if count > 1 {
				// Not sure this would ever happen, but just in case
				fmt.Printf("Found %d entries with this appId\n", count)
				return nil
			} else {
				return nil
			}
		}
	}
	return r
}
