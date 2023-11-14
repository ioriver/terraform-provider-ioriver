package provider

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

	var obj interface{}

	mutex.Lock()
	if !doUpdate {
		obj, err = r.create(client, newObj)
	} else {
		obj, err = r.update(client, newObj)
	}
	mutex.Unlock()

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
			tflog.Info(ctx, fmt.Sprintf("Object not found"))
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

	mutex.Lock()
	updatedObj, err := r.update(client, obj)
	mutex.Unlock()

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

	mutex.Lock()
	err := r.delete(client, id)
	mutex.Unlock()

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

// converts pascal case to snake case
func structFieldToResourceFieldName(input string) string {
	var result []rune

	for i, char := range input {
		if unicode.IsUpper(char) {
			if i > 0 && input[i-1] != '_' {
				// Insert underscore before uppercase letter if not at the beginning
				result = append(result, '_')
			}
			// Convert uppercase letter to lowercase
			char = unicode.ToLower(char)
		}
		result = append(result, char)
	}

	return string(result)
}

func isPremitive(target string) bool {
	premitiveTypes := []string{"string", "int", "boolean"}

	for _, item := range premitiveTypes {
		if item == target {
			return true
		}
	}
	return false
}

func objToResourceBase(obj interface{}, d *schema.ResourceData) diag.Diagnostics {

	objType := reflect.TypeOf(obj).Elem()
	if objType.Kind() != reflect.Struct {
		err := fmt.Errorf("Object type %s is not a struct", objType)
		return diag.FromErr(err)
	}

	objValue := reflect.ValueOf(obj).Elem()

	d.SetId(objValue.FieldByName("Id").String())

	for i := 0; i < objType.NumField(); i++ {

		objField := objType.Field(i)

		if isPremitive(objField.Type.String()) {
			fieldName := objField.Name
			if fieldName == "Id" {
				continue
			}

			resourceField := structFieldToResourceFieldName(fieldName)
			if err := d.Set(resourceField, objValue.FieldByName(fieldName).Interface()); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	return nil
}

// setObjField sets the value of a field in a struct.
func setObjField(obj interface{}, fieldName string, value interface{}) error {
	// Address optional resource fields
	if value == nil {
		return nil
	}

	val := reflect.ValueOf(obj).Elem()
	field := val.FieldByName(fieldName)

	// Check if the field is valid (exists and can be set)
	if !field.IsValid() || !field.CanSet() {
		return fmt.Errorf("Field %s not found or cannot be set", fieldName)
	}

	// Set the field value with the provided value
	if field.Type() == reflect.TypeOf(value) {
		field.Set(reflect.ValueOf(value))
		return nil
	}

	return fmt.Errorf("Value type %T does not match field type %s", value, field.Type())
}

func baseResourceToObj(obj interface{}, d *schema.ResourceData) (interface{}, error) {

	objType := reflect.TypeOf(obj).Elem()
	if objType.Kind() != reflect.Struct {
		err := fmt.Errorf("Object type %s is not a struct", objType)
		return nil, err
	}

	// set each one of the object fields from the resource
	for i := 0; i < objType.NumField(); i++ {

		objField := objType.Field(i)

		if isPremitive(objField.Type.String()) {
			fieldName := objField.Name
			if fieldName == "Id" {
				continue
			}

			resourceField := structFieldToResourceFieldName(fieldName)
			if err := setObjField(obj, fieldName, d.Get(resourceField)); err != nil {
				return nil, err
			}
		}
	}

	return obj, nil
}
