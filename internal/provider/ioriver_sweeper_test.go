package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	ioriver "github.com/ioriver/ioriver-go"
)

func TestMain(m *testing.M) {
	resource.TestMain(m)
}

func sharedClient() (*ioriver.IORiverClient, error) {
	client := ioriver.NewClient(os.Getenv("IORIVER_API_TOKEN"))

	apiEndpoint := ""
	if apiEndpoint != "" {
		client.EndpointUrl = apiEndpoint
	}
	client.TerraformVersion = "test"

	return client, nil
}
