package maz

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/queone/utl"
)

// Prints application object in YAML-like format
func PrintApp(x map[string]interface{}, z Bundle) {
	if x == nil {
		return
	}
	id := utl.Str(x["id"])

	// Print the most important attributes first
	list := []string{"id", "displayName", "appId"}
	for _, i := range list {
		v := utl.Str(x[i])
		if v != "" { // Only print non-null attributes
			fmt.Printf("%s: %s\n", utl.Blu(i), utl.Gre(v))
		}
	}

	// Print certificates keys
	if x["keyCredentials"] != nil {
		PrintCertificateList(x["keyCredentials"].([]interface{}))
	}

	// Print secret list & expiry details, not actual secretText (which cannot be retrieve anyway)
	if x["passwordCredentials"] != nil {
		PrintSecretList(x["passwordCredentials"].([]interface{}))
	}

	// Print federated IDs
	//url := ConstMgUrl + "/v1.0/applications/" + id + "/federatedIdentityCredentials"
	url := ConstMgUrl + "/beta/applications/" + id + "/federatedIdentityCredentials"
	r, statusCode, _ := ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil {
		fedCreds := r["value"].([]interface{})
		if len(fedCreds) > 0 {
			fmt.Println(utl.Blu("federated_ids") + ":")
			for _, i := range fedCreds {
				a := i.(map[string]interface{})
				iId := utl.Gre(utl.Str(a["id"]))
				name := utl.Gre(utl.Str(a["name"]))
				subject := utl.Gre(utl.Str(a["subject"]))
				issuer := utl.Gre(utl.Str(a["issuer"]))
				fmt.Printf("  %-36s  %-30s  %-40s  %s\n", iId, name, subject, issuer)
			}
		}
	}

	// Print owners
	url = ConstMgUrl + "/beta/applications/" + id + "/owners"
	r, statusCode, _ = ApiGet(url, z, nil)
	if statusCode == 200 && r != nil && r["value"] != nil {
		PrintOwners(r["value"].([]interface{}))
	}

	// Print API permissions that have been setup as *REQUIRED* for this application
	// - https://learn.microsoft.com/en-us/entra/identity-platform/app-objects-and-service-principals?tabs=browser
	// - https://learn.microsoft.com/en-us/entra/identity-platform/permissions-consent-overview
	// Just look under the object's 'requiredResourceAccess' attribute
	if x["requiredResourceAccess"] != nil && len(x["requiredResourceAccess"].([]interface{})) > 0 {
		fmt.Printf(utl.Blu("requiredResourceAccess") + ":\n")
		APIs := x["requiredResourceAccess"].([]interface{}) // Assert to JSON array
		for _, a := range APIs {
			api := a.(map[string]interface{})
			// Getting this API's name and permission value such as Directory.Read.All is a 2-step process:
			// 1) Get all the roles for given API and put their id/value pairs in a map, then
			// 2) Use that map to enumerate and print them

			// Let's drill down into the permissions for this API
			if api["resourceAppId"] == nil {
				fmt.Printf("  %-50s %s\n", "Unknown API", "Missing resourceAppId")
				continue // Skip this API, move on to next one
			}
			resAppId := utl.Str(api["resourceAppId"])

			// Get this API's SP object with all relevant attributes
			params := map[string]string{"$filter": "appId eq '" + resAppId + "'"}
			url := ConstMgUrl + "/beta/servicePrincipals"
			r, _, _ := ApiGet(url, z, params)
			ApiErrorCheck("GET", url, utl.Trace(), r) // TODO: Get rid of this by using StatuCode checks, etc
			// Result is a list because this could be a multi-tenant app, having multiple SPs
			if r["value"] == nil {
				fmt.Printf("  %-50s %s\n", resAppId, "Unable to get Resource App object. Skipping this API.")
				continue
			}
			// TODO: Handle multiple SPs

			SPs := r["value"].([]interface{})
			if len(SPs) > 1 {
				utl.Die("  %-50s %s\n", resAppId, "Error. Multiple SPs for this AppId. Aborting.")
			}
			sp := SPs[0].(map[string]interface{}) // Currently only handling the expected single-tenant entry

			// 1. Put all API role id:name pairs into roleMap list
			roleMap := make(map[string]string)
			if sp["appRoles"] != nil { // These are for Application types
				for _, i := range sp["appRoles"].([]interface{}) { // Iterate through all roles
					role := i.(map[string]interface{})
					//utl.PrintJsonColor(role) // DEBUG
					if role["id"] != nil && role["value"] != nil {
						roleMap[utl.Str(role["id"])] = utl.Str(role["value"]) // Add entry to map
					}
				}
			}
			if sp["publishedPermissionScopes"] != nil { // These are for Delegated types
				for _, i := range sp["publishedPermissionScopes"].([]interface{}) {
					role := i.(map[string]interface{})
					//utl.PrintJsonColor(role) // DEBUG
					if role["id"] != nil && role["value"] != nil {
						roleMap[utl.Str(role["id"])] = utl.Str(role["value"])
					}
				}
			}
			if len(roleMap) < 1 {
				fmt.Printf("  %-50s %s\n", resAppId, "Error getting list of appRoles.")
				continue
			}

			// 2. Parse this app permissions, and use roleMap to display permission value
			if api["resourceAccess"] != nil && len(api["resourceAccess"].([]interface{})) > 0 {
				Perms := api["resourceAccess"].([]interface{})
				//utl.PrintJsonColor(Perms)             // DEBUG
				apiName := utl.Str(sp["displayName"]) // This API's name
				for _, i := range Perms {             // Iterate through perms
					perm := i.(map[string]interface{})
					pid := utl.Str(perm["id"]) // JSON string
					var pType string = "?"
					if utl.Str(perm["type"]) == "Role" {
						pType = "Application"
					} else {
						pType = "Delegated"
					}
					fmt.Printf("  %s%s  %s%s  %s\n", utl.Gre(apiName), utl.PadSpaces(40, len(apiName)),
						utl.Gre(pType), utl.PadSpaces(14, len(pType)), utl.Gre(roleMap[pid]))
				}
			} else {
				fmt.Printf("  %-50s %s\n", resAppId, "Error getting list of appRoles.")
			}
		}
	}
}

// Creates/adds a secret to the given application
func AddAppSecret(uuid, displayName, expiry string, z Bundle) {
	if !utl.ValidUuid(uuid) {
		utl.Die("Invalid App UUID.\n")
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
	url := ConstMgUrl + "/v1.0/applications/" + uuid + "/addPassword"
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

// Removes a secret from the given application
func RemoveAppSecret(uuid, keyId string, z Bundle) {
	if !utl.ValidUuid(uuid) {
		utl.Die("App UUID is not a valid UUID.\n")
	}
	if !utl.ValidUuid(keyId) {
		utl.Die("Secret ID is not a valid UUID.\n")
	}

	// Get app, display details and secret, and prompt for delete confirmation
	x := GetAzAppByUuid(uuid, z)
	if x == nil || x["id"] == nil {
		utl.Die("There's no App with this UUID.\n")
	}
	pwdCreds := x["passwordCredentials"].([]interface{})
	if pwdCreds == nil || len(pwdCreds) < 1 {
		utl.Die("App object has no secrets.\n")
	}
	var a map[string]interface{} = nil // Target keyId, Secret ID to be deleted
	for _, i := range pwdCreds {
		targetKeyId := i.(map[string]interface{})
		if utl.Str(targetKeyId["keyId"]) == keyId {
			a = targetKeyId
			break
		}
	}
	if a == nil {
		utl.Die("App object does not have this Secret ID.\n")
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
		url := ConstMgUrl + "/v1.0/applications/" + uuid + "/removePassword"
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

// Retrieves count of all applications in local cache file
func AppsCountLocal(z Bundle) int64 {
	var cachedList []interface{} = nil
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_applications."+ConstCacheFileExtension)
	if utl.FileUsable(cacheFile) {
		rawList, _ := utl.LoadFileJsonGzip(cacheFile)
		if rawList != nil {
			cachedList = rawList.([]interface{})
			return int64(len(cachedList))
		}
	}
	return 0
}

// Retrieves count of all applications in Azure tenant
func AppsCountAzure(z Bundle) int64 {
	z.MgHeaders["ConsistencyLevel"] = "eventual"
	//url := ConstMgUrl + "/v1.0/applications/$count"
	url := ConstMgUrl + "/beta/applications/$count"
	r, _, _ := ApiGet(url, z, nil)
	ApiErrorCheck("GET", url, utl.Trace(), r)
	if r["value"] != nil {
		return r["value"].(int64) // Expected result is a single int64 value for the count
	}
	return 0
}

// Returns an id:name map of all applications
func GetIdMapApps(z Bundle) (nameMap map[string]string) {
	nameMap = make(map[string]string)
	apps := GetMatchingApps("", false, z) // false = don't force a call to Azure
	// By not forcing an Azure call we're opting for cache speed over id:name map accuracy
	for _, i := range apps {
		x := i.(map[string]interface{})
		if x["id"] != nil && x["displayName"] != nil {
			nameMap[utl.Str(x["id"])] = utl.Str(x["displayName"])
		}
	}
	return nameMap
}

// Gets all applications matching on 'filter'. Return entire list if filter is empty ""
func GetMatchingApps(filter string, force bool, z Bundle) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_applications."+ConstCacheFileExtension)
	cacheFileAge := utl.FileAge(cacheFile)
	if utl.InternetIsAvailable() && (force || cacheFileAge == 0 || cacheFileAge > ConstMgCacheFileAgePeriod) {
		// If Internet is available AND (force was requested OR cacheFileAge is zero (meaning does not exist)
		// OR it is older than ConstMgCacheFileAgePeriod) then query Azure directly to get all objects
		// and show progress while doing so (true = verbose below)
		list = GetAzApps(z, true)
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
		// Match against relevant strings within application JSON object (Note: Not all attributes are maintained)
		if !utl.ItemInList(id, ids) && utl.StringInJson(x, filter) {
			matchingList = append(matchingList, x)
			ids = append(ids, id)
		}
	}
	return matchingList
}

// Gets all applications from Azure and sync to local cache. Shows progress if verbose = true
func GetAzApps(z Bundle, verbose bool) (list []interface{}) {
	cacheFile := filepath.Join(z.ConfDir, z.TenantId+"_applications."+ConstCacheFileExtension)
	deltaLinkFile := filepath.Join(z.ConfDir, z.TenantId+"_applications_deltaLink."+ConstCacheFileExtension)

	baseUrl := ConstMgUrl + "/beta/applications"
	// Get delta updates only if/when below attributes in $select are modified
	selection := "?$select=displayName,appId,requiredResourceAccess,passwordCredentials"
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

// Gets application by its Object UUID or by its appId, with all attributes
func GetAzAppByUuid(uuid string, z Bundle) map[string]interface{} {
	baseUrl := ConstMgUrl + "/beta/applications"
	selection := "?$select=*"
	url := baseUrl + "/" + uuid + selection // First search is for direct Object Id
	r, _, _ := ApiGet(url, z, nil)
	if r != nil && r["error"] != nil {
		// Second search is for this app's application Client Id
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
