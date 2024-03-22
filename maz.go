// Package maz is a library of functions for interacting with essential Azure APIs via
// REST calls. Currently it supports two APIs, the Azure Resource Management (ARM) API
// and the MS Graph API, but can be extended to support additional APIs. This package
// obviously also includes code to get an Azure JWT token using the MSAL library, to
// then use against either the 2 currently supported Azure APIs.
package maz

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/queone/utl"
)

const (
	ConstAuthUrl = "https://login.microsoftonline.com/"
	ConstMgUrl   = "https://graph.microsoft.com"
	ConstAzUrl   = "https://management.azure.com"

	ConstAzPowerShellClientId = "1950a258-227b-4e31-a9cf-717495945fc2" // 'Microsoft Azure PowerShell' ClientId
	//ConstAzPowerShellClientId = "04b07795-8ddb-461a-bbee-02f9e1bf7b46" // 'Microsoft Azure CLI' ClientId
	// Interactive login can use either of above ClientIds. See below references:
	//   - https://learn.microsoft.com/en-us/troubleshoot/azure/active-directory/verify-first-party-apps-sign-in
	//   - https://stackoverflow.com/questions/30454771/how-does-azure-powershell-work-with-username-password-based-auth

	rUp = "\x1B[2K\r" // Used to print in previous line
	// See https://stackoverflow.com/questions/1508490/erase-the-current-printed-console-line
	ConstCacheFileExtension   = "gz"
	ConstMgCacheFileAgePeriod = 1800  // Half hour
	ConstAzCacheFileAgePeriod = 86400 // One day
)

var (
	mazTypes     = []string{"d", "a", "s", "u", "g", "sp", "ap", "ad"}
	mazTypesLong = map[string]string{
		"d":  "RBAC Role Definition",
		"a":  "RBAC Role Assignment",
		"s":  "Azure Subscription",
		"u":  "Azure AD User",
		"g":  "Azure AD Group",
		"sp": "Service Principal",
		"ap": "Registered Application",
		"ad": "Azure AD Role",
	}
	eVars = map[string]string{
		"MAZ_TENANT_ID":     "",
		"MAZ_USERNAME":      "",
		"MAZ_INTERACTIVE":   "",
		"MAZ_CLIENT_ID":     "",
		"MAZ_CLIENT_SECRET": "",
		"MAZ_MG_TOKEN":      "",
		"MAZ_AZ_TOKEN":      "",
	}
)

type Bundle struct {
	ConfDir      string // Directory where utility will store all its file
	CredsFile    string
	TokenFile    string
	TenantId     string
	ClientId     string
	ClientSecret string
	Interactive  bool
	Username     string
	AuthorityUrl string
	MgToken      string // This and below to support MS Graph API
	MgHeaders    map[string]string
	AzToken      string // This and below to support Azure Resource Management API
	AzHeaders    map[string]string
	// To support other future APIs, those token/headers pairs can be added here
}

// Dumps configured login values
func DumpLoginValues(z Bundle) {
	fmt.Printf("%s: %s  # Config and cache directory\n", utl.Blu("config_dir"), utl.Gre(z.ConfDir))

	fmt.Printf("%s:\n", utl.Blu("config_env_variables"))
	fmt.Println("  # 1. MS Graph and Azure ARM tokens can be supplied directly via MAZ_MG_TOKEN and")
	fmt.Println("  #    MAZ_AZ_TOKEN environment variables, and they have the highest precedence.")
	fmt.Println("  #    Note, MAZ_TENANT_ID is still required when using these 2.")
	fmt.Println("  # 2. Credentials supplied via environment variables have precedence over those")
	fmt.Println("  #    provided via credentials file.")
	fmt.Println("  # 3. The MAZ_USERNAME + MAZ_INTERACTIVE combo have priority over the MAZ_CLIENT_ID")
	fmt.Println("  #    + MAZ_CLIENT_SECRET combination.")
	fmt.Printf("  %s: %s\n", utl.Blu("MAZ_TENANT_ID"), utl.Gre(os.Getenv("MAZ_TENANT_ID")))
	fmt.Printf("  %s: %s\n", utl.Blu("MAZ_USERNAME"), utl.Gre(os.Getenv("MAZ_USERNAME")))
	fmt.Printf("  %s: %s\n", utl.Blu("MAZ_INTERACTIVE"), utl.Mag(os.Getenv("MAZ_INTERACTIVE")))
	fmt.Printf("  %s: %s\n", utl.Blu("MAZ_CLIENT_ID"), utl.Gre(os.Getenv("MAZ_CLIENT_ID")))
	fmt.Printf("  %s: %s\n", utl.Blu("MAZ_CLIENT_SECRET"), utl.Gre(os.Getenv("MAZ_CLIENT_SECRET")))
	fmt.Printf("  %s: %s\n", utl.Blu("MAZ_MG_TOKEN"), utl.Gre(os.Getenv("MAZ_MG_TOKEN")))
	fmt.Printf("  %s: %s\n", utl.Blu("MAZ_AZ_TOKEN"), utl.Gre(os.Getenv("MAZ_AZ_TOKEN")))
	fmt.Printf("%s:\n", utl.Blu("config_creds_file"))
	filePath := filepath.Join(z.ConfDir, z.CredsFile)
	fmt.Printf("  %s: %s\n", utl.Blu("file_path"), utl.Gre(filePath))
	credsRaw, err := utl.LoadFileYaml(filePath)
	if err != nil {
		utl.Die(utl.Red("  Credentials file does not exists yet.\n"))
	}
	creds := credsRaw.(map[string]interface{})
	fmt.Printf("  %s: %s\n", utl.Blu("tenant_id"), utl.Gre(utl.Str(creds["tenant_id"])))
	if strings.ToLower(utl.Str(creds["interactive"])) == "true" {
		fmt.Printf("  %s: %s\n", utl.Blu("username"), utl.Gre(utl.Str(creds["username"])))
		fmt.Printf("  %s: %s\n", utl.Blu("interactive"), utl.Mag("true"))
	} else {
		fmt.Printf("  %s: %s\n", utl.Blu("client_id"), utl.Gre(utl.Str(creds["client_id"])))
		fmt.Printf("  %s: %s\n", utl.Blu("client_secret"), utl.Gre(utl.Str(creds["client_secret"])))
	}
	os.Exit(0)
}

// Sets up credentials file for interactive login
func SetupInterativeLogin(z Bundle) {
	filePath := filepath.Join(z.ConfDir, z.CredsFile) // credentials.yaml
	if !utl.ValidUuid(z.TenantId) {
		utl.Die("Error. TENANT_ID is an invalid UUID.\n")
	}
	content := fmt.Sprintf("%-14s %s\n%-14s %s\n%-14s %s\n", "tenant_id:", z.TenantId, "username:", z.Username, "interactive:", "true")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil { // Write string to file
		panic(err.Error())
	}
	fmt.Printf("Updated %s file\n", utl.Gre(filePath))
	os.Exit(0)
}

// Sets up credentials file for client_id + secret login
func SetupAutomatedLogin(z Bundle) {
	filePath := filepath.Join(z.ConfDir, z.CredsFile) // credentials.yaml
	if !utl.ValidUuid(z.TenantId) {
		utl.Die("Error. TENANT_ID is an invalid UUID.\n")
	}
	if !utl.ValidUuid(z.ClientId) {
		utl.Die("Error. CLIENT_ID is an invalid UUID.\n")
	}
	content := fmt.Sprintf("%-14s %s\n%-14s %s\n%-14s %s\n", "tenant_id:", z.TenantId, "client_id:", z.ClientId, "client_secret:", z.ClientSecret)
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil { // Write string to file
		panic(err.Error())
	}
	fmt.Printf("Updated %s file\n", utl.Gre(filePath))
	os.Exit(0)
}

// Gets credentials from OS environment variables (which take precedence), or from the
// credentials file.
func SetupCredentials(z *Bundle) Bundle {
	usingEnv := false // Assume environment variables are not being used
	for k := range eVars {
		eVars[k] = os.Getenv(k) // Read all MAZ_* environment variables
		if eVars[k] != "" {
			usingEnv = true // If any are set, environment variable login/token is true
		}
	}
	if usingEnv {
		// Getting from OS environment variables
		z.TenantId = eVars["MAZ_TENANT_ID"]
		if !utl.ValidUuid(z.TenantId) {
			utl.Die("[MAZ_TENANT_ID] tenant_id '%s' is not a valid UUID\n", z.TenantId)
		}
		z.MgToken = eVars["MAZ_MG_TOKEN"]
		z.AzToken = eVars["MAZ_AZ_TOKEN"]
		// Let's assume tokens for each of the 2 APIs have been supplied
		if !TokenValid(z.AzToken) && !TokenValid(z.MgToken) {
			// If they are both not valid, then we'll process the other variables
			z.Interactive, _ = strconv.ParseBool(utl.Str(eVars["MAZ_INTERACTIVE"]))
			if z.Interactive {
				z.Username = strings.ToLower(utl.Str(eVars["MAZ_USERNAME"]))
				if z.ClientId != "" || z.ClientSecret != "" {
					fmt.Println("Warning: ", utl.Yel(""))
				}
			} else {
				z.ClientId = utl.Str(eVars["MAZ_CLIENT_ID"])
				if !utl.ValidUuid(z.ClientId) {
					utl.Die("[MAZ_CLIENT_ID] client_id '%s' is not a valid UUID\n", z.ClientId)
				}
				z.ClientSecret = utl.Str(eVars["MAZ_CLIENT_SECRET"])
				if z.ClientSecret == "" {
					utl.Die("[MAZ_CLIENT_SECRET] client_secret is blank\n")
				}
			}
		} // ... else it gets the Tenant Id from the valid tokens
	} else {
		// Getting from credentials file
		filePath := filepath.Join(z.ConfDir, z.CredsFile) // credentials.yaml
		if utl.FileNotExist(filePath) && utl.FileSize(filePath) < 1 {
			utl.Die("Missing credentials file: " + filePath + "\n" +
				"Re-run program to set up the appropriate login credentials.\n")
		}
		credsRaw, err := utl.LoadFileYaml(filePath)
		if err != nil {
			utl.Die("[%s] %s\n", filePath, err)
		}
		creds := credsRaw.(map[string]interface{})
		z.TenantId = utl.Str(creds["tenant_id"])
		if !utl.ValidUuid(z.TenantId) {
			utl.Die("[%s] tenant_id '%s' is not a valid UUID\n", filePath, z.TenantId)
		}
		z.Interactive, _ = strconv.ParseBool(utl.Str(creds["interactive"]))
		if z.Interactive {
			z.Username = strings.ToLower(utl.Str(creds["username"]))
		} else {
			z.ClientId = utl.Str(creds["client_id"])
			if !utl.ValidUuid(z.ClientId) {
				utl.Die("[%s] client_id '%s' is not a valid UUID\n", filePath, z.ClientId)
			}
			z.ClientSecret = utl.Str(creds["client_secret"])
			if z.ClientSecret == "" {
				utl.Die("[%s] client_secret is blank\n", filePath)
			}
		}
	}
	return *z
}

// Initializes the necessary global variables, acquires all API tokens, and sets them up for use.
func SetupApiTokens(z *Bundle) Bundle {
	*z = SetupCredentials(z) // Sets up tenant ID, client ID, authentication method, etc

	// Currently supporting calls for 2 different APIs (Azure Resource Management (ARM) and MS Graph), so each needs its own
	// separate token. The Microsoft identity platform does not allow using same token for multiple resources at once.
	// See https://learn.microsoft.com/en-us/azure/active-directory/develop/msal-net-user-gets-consent-for-multiple-resources

	if !TokenValid(z.AzToken) && !TokenValid(z.MgToken) {
		// If API tokens have *both* not been supplied via environment variables, let's go ahead and get them
		// via the other supported methods.

		z.AuthorityUrl = ConstAuthUrl + z.TenantId

		// Get a token for ARM access
		azScope := []string{ConstAzUrl + "/.default"}
		// Appending '/.default' allows using all static and consented permissions of the identity in use
		// See https://learn.microsoft.com/en-us/azure/active-directory/develop/msal-v1-app-scopes
		if z.Interactive {
			// Get token interactively
			z.AzToken, _ = GetTokenInteractively(azScope, z.ConfDir, z.TokenFile, z.AuthorityUrl, z.Username)
		} else {
			// Get token with clientId + Secret
			z.AzToken, _ = GetTokenByCredentials(azScope, z.ConfDir, z.TokenFile, z.AuthorityUrl, z.ClientId, z.ClientSecret)
		}

		// Get a token for MS Graph access
		mgScope := []string{ConstMgUrl + "/.default"}
		if z.Interactive {
			z.MgToken, _ = GetTokenInteractively(mgScope, z.ConfDir, z.TokenFile, z.AuthorityUrl, z.Username)
		} else {
			z.MgToken, _ = GetTokenByCredentials(mgScope, z.ConfDir, z.TokenFile, z.AuthorityUrl, z.ClientId, z.ClientSecret)
		}

		// Support for other APIs can be added here in the future ...
	}

	// Setup the base API headers; token + content type
	z.AzHeaders = map[string]string{"Authorization": "Bearer " + z.AzToken, "Content-Type": "application/json"}
	z.MgHeaders = map[string]string{"Authorization": "Bearer " + z.MgToken, "Content-Type": "application/json"}

	return *z
}
