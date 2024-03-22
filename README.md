# maz
This is a Go library package module for managing Microsoft Azure resource and security objects. Please review <https://que.tips/azure/> to better understand what is meant here by *resource* and *security* objects. Essentially this is a library that provides basic MSAL authentication and token creation to allow principals to call the two primary Azure APIs, the Azure Resource Managment (ARM) API and the MS Graph API. Other APIs could be added in the future.

## Getting Started
1. Any program wanting to use this library module can simply import it, then instantiate a variable
of type `maz.Bundle` to manage the interaction. For example: 

```go
import (
    "github.com/queone/maz"
)
z := maz.Bundle{
    ConfDir:      "",                   // Set up later, see example below
    CredsFile:    "credentials.yaml",
    TokenFile:    "accessTokens.json",
    TenantId:     "",
    ClientId:     "",
    ClientSecret: "",
    Interactive:  false,
    Username:     "",
    AuthorityUrl: "",                   // Set up later with maz.ConstAuthUrl + z.TenantId (see const block in maz.go)
    MgToken:      "",                   // Set up below 4 later with function maz.SetupApiTokens()
    MgHeaders:    map[string]string{},
    AzToken:      "",
    AzHeaders:    map[string]string{},  
}
// Then update the variables within the Bundle, to set up configuration directory
z.ConfDir = filepath.Join(os.Getenv("HOME"), "." + prgname)
if utl.FileNotExist(z.ConfDir) {
    if err := os.Mkdir(z.ConfDir, 0700); err != nil {
        panic(err.Error())
    }
}
```

2. Then call `maz.SetupInterativeLogin(z)` or `maz.SetupAutomatedLogin(z)` to setup the credentials file accordingly.
3. Then call `z := maz.SetupApiTokens(*z)` to acquire the respective API tokens, web headers, and other variables.
4. Now call whatever MS Graph and Azure Resource API functions you want by passing and using the `z` variables,
with its `z.mgHeaders` and/or `z.azHeaders` attributes, and so on.

## Login Credentials
There are four (4) different ways to set up the login credentials to use this library module. All four ways required
three (3) special attributes:

|#|Type|Method|Details|
|-|-|-|-|
|1|Interactive|Config file|Set up attributes via `~/.maz/credentials.yaml` file|
|2|Interactive|Environment variables|Set up attributes via environment variables (**OVERIDES config file**)|
|3|Automated|Config file|Set up attributes via `~/.maz/credentials.yaml` file|
|4|Automated|Environment variables|Set up attributes via environment variables (**OVERIDES config file**)|

1. *Interactive via config file*: The calling utility sets up a way to allow setting up the `~/.maz/credentials.yaml` file with
   the 3 special attributes. For example, the [azm CLI utility](https://github.com/queone/azm) does this via the `-id`
   switch, to _Set up MSAL interactive browser popup login_:
   ```
   azm -id 3f050090-20b0-40a0-a060-c05060104010 user1@domain.io
   ```
   Above will populate the `~/.maz/credentials.yaml` file as follows:
   ```yaml
   tenant_id: 3f050090-20b0-40a0-a060-c05060104010
   username: user1@domain.io
   interactive: true
   ```
   From then on the `azm` utility will use above credentials to interact with the `maz` library to perform all its functions.
2. *Interactive via environment variables*: The calling utility will instead use the `os.Getenv("VAR")` function to look for
   the following 3 special environment variables:
   ```bsh
   MAZ_TENANT_ID=3f050090-20b0-40a0-a060-c05060104010
   MAZ_USERNAME=user1@domain.io
   MAZ_INTERACTIVE=true
   ```
   Above values take **precedence** and **OVERIDE** any existing config `~/.maz/credentials.yaml` file values. 
3. *Automated via config file*: The calling utility sets up a way to allow setting up the `~/.maz/credentials.yaml` file with
   the 3 special attributes. For example, the [azm CLI utility](https://github.com/queone/azm) does this via the `-id`
   switch, to _Set up MSAL automated ClientId + Secret login_:
   ```
   azm -id 3f050090-20b0-40a0-a060-c05060104010 f1110121-7111-4171-a181-e1614131e181 ACB8c~HdLejfQGiHeI9LUKgNOODPQRISNTmVLX_i
   ```
   Above will populate the `~/.maz/credentials.yaml` file as follows:
   ```yaml
   tenant_id: 3f050090-20b0-40a0-a060-c05060104010
   client_id: f1110121-7111-4171-a181-e1614131e181
   client_secret: ACB8c~HdLejfQGiHeI9LUKgNOODPQRISNTmVLX_i
   ```
   From then on the `azm` utility will use above credentials to interact with the `maz` library to perform all its functions.
4. *Automated via environment variables*: The calling utility will instead use the `os.Getenv("VAR")` function to look for
   the following 3 special environment variables
   ```bsh
   MAZ_TENANT_ID=3f050090-20b0-40a0-a060-c05060104010
   MAZ_CLIENT_ID=f1110121-7111-4171-a181-e1614131e181
   MAZ_CLIENT_SECRET=ACB8c~HdLejfQGiHeI9LUKgNOODPQRISNTmVLX_i
   ```
   Above values take **precedence** and **OVERIDE** any existing config `~/.maz/credentials.yaml` file values. 

The benefit of using environment variables is to be able to override an existing `credentials.yaml` file, and to specify different credentials, as well as being able to use different credentials from different shell sessions _on the same host_. They also allow utilities written with this library to be used in continuous delivery and other types of automation.

*NOTE*: If all four `MAZ_USERNAME`, `MAZ_INTERACTIVE`, `MAZ_CLIENT_ID`, and `MAZ_CLIENT_SECRET` are properly define, then _precedence_ is given to the Username Interactive login. To force a ClientID ClientSecret login via environment variables, you must ensure the first two are `unset` in the current shell.

## Functions
TODO: List of all available functions?
- **maz.SetupInterativeLogin**: This functions allows you to set up the`~/.maz/credentials.yaml` file for interactive Azure login.
- ...
