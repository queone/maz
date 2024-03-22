package maz

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/queone/utl"
)

// Creates specfile skeleton/scaffold files
func CreateSkeletonFile(t string) {
	pwd, err := os.Getwd()
	if err != nil {
		utl.Die(utl.Trace() + "Error: Getting CWD\n")
	}
	fileName, fileContent := "init-file-name.extension", []byte("init-file-content\n")
	switch t {
	case "d":
		fileName = "role-definition.yaml"
		fileContent = []byte("properties:\n" +
			"  roleName: My RBAC Role\n" +
			"  description: Description of what this role does.\n" +
			"  assignableScopes:\n" +
			"    # Recommendation: Always define at highest point in hierarchy, the Tenant Root Group.\n" +
			"    - /providers/Microsoft.Management/managementGroups/3f550b9f-8888-7777-ad61-111199992222\n" +
			"  permissions:\n" +
			"    - actions:\n" +
			"        - Microsoft.DevCenter/projects/*/read\n" +
			"        - '*/read'       # Wrap leading asterik entries in single-quotes\n" +
			"      notActions:\n" +
			"        - Microsoft.DevCenter/projects/pools/read\n" +
			"      dataActions:\n" +
			"        - Microsoft.KeyVault/vaults/secrets/*\n" +
			"      notDataActions:\n" +
			"        - Microsoft.CognitiveServices/accounts/LUIS/apps/delete\n")
	case "dj":
		fileName = "role-definition.json"
		fileContent = []byte("{\n" +
			"  \"properties\": {\n" +
			"    \"roleName\": \"My RBAC Role\",\n" +
			"    \"description\": \"Description of what this role does.\",\n" +
			"    \"assignableScopes\": [\n" +
			"      \"/providers/Microsoft.Management/managementGroups/3f550b9f-8888-7777-ad61-111199992222\"\n" +
			"    ],\n" +
			"    \"permissions\": [\n" +
			"      {\n" +
			"        \"actions\": [\n" +
			"          \"Microsoft.DevCenter/projects/*/read\"\n" +
			"        ],\n" +
			"        \"notActions\": [\n" +
			"          \"Microsoft.DevCenter/projects/pools/read\"\n" +
			"        ],\n" +
			"        \"dataActions\": [\n" +
			"          \"Microsoft.KeyVault/vaults/secrets/*\"\n" +
			"        ],\n" +
			"        \"notDataActions\": [\n" +
			"          \"Microsoft.CognitiveServices/accounts/LUIS/apps/delete\"\n" +
			"        ]\n" +
			"      }\n" +
			"    ]\n" +
			"  }\n" +
			"}\n")
	case "a":
		fileName = "role-assignment.yaml"
		fileContent = []byte("properties:\n" +
			"  principalId: 65c6427a-1111-5555-7777-274d26531314  # Group = \"My Special Group\"\n" +
			"  roleDefinitionId: 2489dfa4-3333-4444-9999-b04b7a1e4ea6  # Role = \"My Special Role\"\n" +
			"  scope: /providers/Microsoft.Management/managementGroups/3f550b9f-8888-7777-ad61-111199992222\n")
	case "aj":
		fileName = "role-assignment.json"
		fileContent = []byte("{\n" +
			"  \"properties\": {\n" +
			"    \"principalId\": \"65c6427a-1111-5555-7777-274d26531314\",\n" +
			"    \"roleDefinitionId\": \"2489dfa4-3333-4444-9999-b04b7a1e4ea6\",\n" +
			"    \"scope\": \"/providers/Microsoft.Management/managementGroups/3f550b9f-8888-7777-ad61-111199992222\"\n" +
			"  }\n" +
			"}\n")
	}
	filePath := filepath.Join(pwd, fileName)
	if utl.FileExist(filePath) {
		utl.Die("Error: File " + fileName + " already exists.\n")
	}
	f, err := os.Create(filePath) // Create the file
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()
	f.Write(fileContent) // Write the content
	os.Exit(0)
}
