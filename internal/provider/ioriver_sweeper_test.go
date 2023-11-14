package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	ioriver "github.com/ioriver/ioriver-go"
)

func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func sharedClient() (*ioriver.IORiverClient, error) {
	client := ioriver.NewClient(os.Getenv("IORIVER_API_TOKEN"))

	// apiEndpoint := GetDefaultFromEnv(APIEndpointEnvVar, "")

	apiEndpoint := ""
	if apiEndpoint != "" {
		client.EndpointUrl = apiEndpoint
	}

	return client, nil
}
