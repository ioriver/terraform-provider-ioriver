package provider

import (
	"github.com/ioriver/ioriver-go"
)

type ServiceWithConfig struct {
	Id           string                 `json:"id,omitempty"`
	Account      string                 `json:"account,omitempty"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Certificates []string               `json:"certificates"`
	ServiceUid   string                 `json:"service_uid,omitempty"`
	Cname        string                 `json:"cname,omitempty"`
	Config       map[string]interface{} `json:"service_config,omitempty"` // Not returned by API, populated separately
}

func CreateServiceWithConfig(client *ioriver.IORiverClient, serviceWithConfig ServiceWithConfig) (*ServiceWithConfig, error) {
	service := ioriver.Service{
		Id:          serviceWithConfig.Id,
		Account:     serviceWithConfig.Account,
		Name:        serviceWithConfig.Name,
		Description: serviceWithConfig.Description,
		Certificate: serviceWithConfig.Certificates[0],
		ServiceUid:  serviceWithConfig.ServiceUid,
		Cname:       serviceWithConfig.Cname,
	}
	serviceConfig := ioriver.ServiceConfig{
		ConfigJSON: serviceWithConfig.Config,
	}

	resp, err := client.CreateServiceWithConfig(service, serviceConfig)
	if err != nil {
		return nil, err
	}

	// Assemble service with service-config
	return GetServiceWithConfig(client, resp.Id)
}

func UpdateServiceWithConfig(client *ioriver.IORiverClient, service ServiceWithConfig) (*ServiceWithConfig, error) {
	// Retrieve config to use fields for the update
	serviceConfigResponse, err := client.GetCurrentServiceConfig(service.Id)
	if err != nil {
		return nil, err
	}

	// Update config - backend uses POST (create) to add a new service config version
	_, err = client.UpdateServiceConfig(service.Id, ioriver.ServiceConfig{
		ParentVersion: serviceConfigResponse.Version,
		Description:   service.Description,
		ConfigJSON:    service.Config,
	})
	if err != nil {
		return nil, err
	}

	// Update service fields: name, description
	serviceReq := ioriver.Service{
		Id:          service.Id,
		Name:        service.Name,
		Description: service.Description,
	}
	_, err = client.UpdateService(serviceReq)
	if err != nil {
		return nil, err
	}

	return GetServiceWithConfig(client, service.Id)
}

func GetServiceWithConfig(client *ioriver.IORiverClient, id string) (*ServiceWithConfig, error) {
	service, err := client.GetService(id)
	if err != nil {
		return nil, err
	}

	// Fetch the current service config separately
	serviceConfigResponse, err := client.GetCurrentServiceConfig(id)
	if err != nil {
		return nil, err
	}

	serviceWithConfig := ServiceWithConfig{
		Id:           service.Id,
		Account:      service.Account,
		Name:         service.Name,
		Description:  service.Description,
		Certificates: []string{service.Certificate},
		ServiceUid:   service.ServiceUid,
		Cname:        service.Cname,
		Config:       serviceConfigResponse.ConfigJSON,
	}

	return &serviceWithConfig, nil
}

func ListServicesWithConfig(client *ioriver.IORiverClient) ([]ServiceWithConfig, error) {
	services, err := client.ListServices()
	if err != nil {
		return nil, err
	}
	servicesWithConfig := make([]ServiceWithConfig, 0, len(services))
	for _, service := range services {
		// Map the service to ServiceWithConfig
		serviceWithConfig := ServiceWithConfig{
			Id:           service.Id,
			Account:      service.Account,
			Name:         service.Name,
			Description:  service.Description,
			Certificates: []string{service.Certificate},
			ServiceUid:   service.ServiceUid,
			Cname:        service.Cname,
			Config:       map[string]interface{}{},
		}
		servicesWithConfig = append(servicesWithConfig, serviceWithConfig)
	}
	return servicesWithConfig, nil
}

func DeleteServiceWithConfig(client *ioriver.IORiverClient, id string) error {
	return client.DeleteService(id)
}
