package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	ioriver "github.com/ioriver/ioriver-go"
)

// protect parallel resource modification using this lock
var mutex sync.Mutex

type Resource interface {
	create(client *ioriver.IORiverClient, newObj interface{}) (interface{}, error)
	read(client *ioriver.IORiverClient, id interface{}) (interface{}, error)
	update(client *ioriver.IORiverClient, obj interface{}) (interface{}, error)
	delete(client *ioriver.IORiverClient, id interface{}) error

	getId(data interface{}) interface{}
	resourceToObj(ctx context.Context, data interface{}) (interface{}, error)
	objToResource(ctx context.Context, obj interface{}) (interface{}, error)
}

func resourceCreate(client *ioriver.IORiverClient, ctx context.Context, req resource.CreateRequest,
	resp *resource.CreateResponse, r Resource, data interface{}, doUpdate bool) interface{} {

	if resp.Diagnostics.HasError() {
		return nil
	}

	newObj, err := r.resourceToObj(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Error creating object from ResourceData", "Unexpected error: "+err.Error())
		return nil
	}

	tflog.Info(ctx, fmt.Sprintf("Creating IORiver object: %#v", newObj))

	var operation func() (interface{}, error)
	if !doUpdate {
		operation = func() (interface{}, error) { return r.create(client, newObj) }
	} else {
		operation = func() (interface{}, error) { return r.update(client, newObj) }
	}
	obj, err := performOperation(func() (interface{}, error) { return operation() })

	if err != nil {
		resp.Diagnostics.AddError("Error creating resource", "Could not create resource, unexpected error: "+err.Error())
		return nil
	}

	resourceModel, err := r.objToResource(ctx, obj)
	if err != nil {
		resp.Diagnostics.AddError("Error creating resource", "Failed to convert IORiver object to resource: "+err.Error())
	}
	return resourceModel
}

func resourceRead(client *ioriver.IORiverClient, ctx context.Context, req resource.ReadRequest,
	resp *resource.ReadResponse, r Resource, data interface{}) interface{} {

	if resp.Diagnostics.HasError() {
		return nil
	}

	obj, err := r.read(client, r.getId(data))

	tflog.Debug(ctx, fmt.Sprintf("Object: %#v", obj))

	if err != nil {
		if strings.Contains(err.Error(), "404 Not Found") {
			tflog.Info(ctx, "Object not found")
			resp.State.RemoveResource(ctx)
			return nil
		}

		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read resource, got error: %s", err))
		return nil
	}

	resourceModel, err := r.objToResource(ctx, obj)
	if err != nil {
		tflog.Error(ctx, fmt.Sprintf("Failed to convert IORiver object to resource, object: %#v", obj))
	}

	return resourceModel
}

func resourceUpdate(client *ioriver.IORiverClient, ctx context.Context, req resource.UpdateRequest,
	resp *resource.UpdateResponse, r Resource, data interface{}) interface{} {

	if resp.Diagnostics.HasError() {
		return nil
	}

	obj, err := r.resourceToObj(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("Error creating object from ResourceData", "Unexpected error: "+err.Error())
		return nil
	}
	tflog.Info(ctx, fmt.Sprintf("Updating IORiver object: %#v", obj))

	updateOp := func() (interface{}, error) { return r.update(client, obj) }
	updatedObj, err := performOperation(func() (interface{}, error) { return updateOp() })

	if err != nil {
		resp.Diagnostics.AddError("Error updating resource", "Could not update resource, unexpected error: "+err.Error())
		return nil
	}

	resourceModel, err := r.objToResource(ctx, updatedObj)
	if err != nil {
		resp.Diagnostics.AddError("Error updating resource", "Failed to convert IORiver object to resource: "+err.Error())
	}
	return resourceModel
}

func resourceDelete(client *ioriver.IORiverClient, ctx context.Context, req resource.DeleteRequest,
	resp *resource.DeleteResponse, r Resource, data interface{}) {

	if resp.Diagnostics.HasError() {
		return
	}

	id := r.getId(data)
	tflog.Info(ctx, fmt.Sprintf("Deleting IORiver object: id %d", id))

	// perform the delete operation
	deleteOp := func() (interface{}, error) { return nil, r.delete(client, id) }
	_, err := performOperation(func() (interface{}, error) { return deleteOp() })

	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete resource, got error: %s", err))
	}
}

func serviceResourceImport(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: service-id,id. Got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service"), idParts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), idParts[1])...)
}

func ConfigureBase(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) *ioriver.IORiverClient {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return nil
	}

	client, ok := req.ProviderData.(*ioriver.IORiverClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ioriver.IORiverClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return nil
	}

	return client
}

// ensures that IO River operations are done sequentially
func performOperation(operation func() (interface{}, error)) (interface{}, error) {
	mutex.Lock()
	defer mutex.Unlock()
	return operation()
}
