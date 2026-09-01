package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	ioriver "github.com/ioriver/ioriver-go"
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
		Id:           serviceWithConfig.Id,
		Account:      serviceWithConfig.Account,
		Name:         serviceWithConfig.Name,
		Description:  serviceWithConfig.Description,
		Certificates: serviceWithConfig.Certificates,
		ServiceUid:   serviceWithConfig.ServiceUid,
		Cname:        serviceWithConfig.Cname,
	}
	serviceConfig := ioriver.ServiceConfig{
		ConfigJSON: serviceWithConfig.Config,
	}

	resp, err := client.CreateServiceWithConfig(service, serviceConfig)
	if err != nil {
		return nil, err
	}

	return GetServiceWithConfig(client, resp.Id)
}

func UpdateServiceWithConfig(ctx context.Context, client *ioriver.IORiverClient, service ServiceWithConfig) (*ServiceWithConfig, error) {
	// Compute the cert diff up front so we can bracket the service update
	// between adds and removes — the service should never have zero valid certs.
	toAdd, toRemove, err := diffServiceCertificates(client, service.Id, service.Certificates)
	if err != nil {
		return nil, err
	}
	tflog.Debug(ctx, fmt.Sprintf("[UpdateServiceWithConfig] serviceId=%s desired=%v toAdd=%v toRemove=%v",
		service.Id, service.Certificates, toAdd, toRemove))

	// 1-for-1 replace: exactly one cert is being swapped out for another.
	// Use ReplaceServiceCertificate (new cert must cover existing domains)
	if len(toAdd) == 1 && len(toRemove) == 1 {
		var oldCertId, existServiceCertId string
		for certId, serviceCertId := range toRemove {
			oldCertId = certId
			existServiceCertId = serviceCertId
		}
		tflog.Debug(ctx, fmt.Sprintf("[UpdateServiceWithConfig] replacing cert: serviceId=%s newCertId=%s oldCertId=%s oldJoinId=%s",
			service.Id, toAdd[0], oldCertId, existServiceCertId))
		resp, err := ioriver.ReplaceServiceCertificate(client, service.Id, toAdd[0], existServiceCertId)
		if err != nil {
			return nil, fmt.Errorf("UpdateServiceWithConfig: %w", err)
		}
		tflog.Debug(ctx, fmt.Sprintf("[UpdateServiceWithConfig] replace result: newJoinId=%s", resp.Id))

		toAdd = nil
		toRemove = nil
	}

	// 1. Add new certificates first (multi-cert expand path).
	for _, certId := range toAdd {
		if _, err := ioriver.AddServiceCertificate(client, service.Id, certId); err != nil {
			return nil, fmt.Errorf("UpdateServiceWithConfig: add cert %s: %w", certId, err)
		}
	}

	// 2. Update config - backend uses POST (create) to add a new service config version.
	serviceConfigResponse, err := client.GetCurrentServiceConfig(service.Id)
	if err != nil {
		return nil, err
	}
	_, err = client.UpdateServiceConfig(service.Id, ioriver.ServiceConfig{
		ParentVersion: serviceConfigResponse.Version,
		Description:   service.Description,
		ConfigJSON:    service.Config,
	})
	if err != nil {
		return nil, err
	}

	// 3. Update service fields: name, description.
	_, err = client.UpdateService(ioriver.Service{
		Id:          service.Id,
		Name:        service.Name,
		Description: service.Description,
	})
	if err != nil {
		return nil, err
	}

	// 4. Remove old certificates last.
	for certId, joinId := range toRemove {
		if err := ioriver.RemoveServiceCertificate(client, service.Id, joinId); err != nil {
			return nil, fmt.Errorf("UpdateServiceWithConfig: remove cert %s: %w", certId, err)
		}
	}

	return GetServiceWithConfig(client, service.Id)
}

// diffServiceCertificates computes which certificates need to be added and
// which need to be removed to reach desiredCertIds.
// Returns:
//
//	toAdd    — cert IDs to POST
//	toRemove — certId → joinId map for certs to DELETE
func diffServiceCertificates(client *ioriver.IORiverClient, serviceId string, desiredCertIds []string) (toAdd []string, toRemove map[string]string, err error) {
	current, err := ioriver.ListServiceCertificates(client, serviceId)
	if err != nil {
		return nil, nil, fmt.Errorf("diffServiceCertificates: list failed: %w", err)
	}

	// certId → joinId for certs currently on the service.
	currentMap := make(map[string]string, len(current))
	for _, sc := range current {
		currentMap[sc.Certificate] = sc.Id
	}

	desiredSet := make(map[string]struct{}, len(desiredCertIds))
	for _, id := range desiredCertIds {
		desiredSet[id] = struct{}{}
	}

	for certId := range desiredSet {
		if _, exists := currentMap[certId]; !exists {
			toAdd = append(toAdd, certId)
		}
	}

	toRemove = make(map[string]string)
	for certId, joinId := range currentMap {
		if _, desired := desiredSet[certId]; !desired {
			toRemove[certId] = joinId
		}
	}

	return toAdd, toRemove, nil
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

	// Fetch the full certificate list from the dedicated endpoint so we
	// correctly handle multi-cert services.
	serviceCerts, err := ioriver.ListServiceCertificates(client, id)
	if err != nil {
		return nil, fmt.Errorf("GetServiceWithConfig: list certificates: %w", err)
	}
	certIds := make([]string, 0, len(serviceCerts))
	for _, sc := range serviceCerts {
		certIds = append(certIds, sc.Certificate)
	}

	serviceWithConfig := ServiceWithConfig{
		Id:           service.Id,
		Account:      service.Account,
		Name:         service.Name,
		Description:  service.Description,
		Certificates: certIds,
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
		serviceCerts, err := ioriver.ListServiceCertificates(client, service.Id)
		if err != nil {
			return nil, fmt.Errorf("ListServicesWithConfig: list certificates for service %s: %w", service.Id, err)
		}
		certIds := make([]string, 0, len(serviceCerts))
		for _, sc := range serviceCerts {
			certIds = append(certIds, sc.Certificate)
		}

		// Map the service to ServiceWithConfig
		serviceWithConfig := ServiceWithConfig{
			Id:           service.Id,
			Account:      service.Account,
			Name:         service.Name,
			Description:  service.Description,
			Certificates: certIds,
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
