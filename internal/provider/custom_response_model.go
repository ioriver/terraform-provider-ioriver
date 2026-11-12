package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type CustomResponseModel struct {
	UUID           types.String `tfsdk:"uuid"`
	Name           types.String `tfsdk:"name"`
	StatusCode     types.Int64  `tfsdk:"status_code"`
	Headers        types.Map    `tfsdk:"headers"`
	Text           types.String `tfsdk:"text"`
	AttachedFileID types.String `tfsdk:"attached_file_id"`
}

func validateCustomResponse(ctx context.Context, model CustomResponseModel) error {
	if model.StatusCode.IsNull() || model.StatusCode.IsUnknown() {
		return nil
	}
	code := model.StatusCode.ValueInt64()
	switch code {
	case 301, 302, 303, 307, 308:
		if model.Headers.IsUnknown() {
			return nil
		}
		if !model.Headers.IsNull() {
			for name, value := range model.Headers.Elements() {
				if !strings.EqualFold(strings.TrimSpace(name), "Location") {
					continue
				}
				if value.IsUnknown() {
					return nil
				}
				header, ok := value.(types.String)
				if !ok {
					return fmt.Errorf("Location header must be a string")
				}
				if !header.IsNull() && strings.TrimSpace(header.ValueString()) != "" {
					return nil
				}
			}
		}
		return fmt.Errorf("Location header is required for redirect status codes 301, 302, 303, 307, and 308")
	}
	return nil
}

func CustomResponseAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"uuid": types.StringType, "name": types.StringType, "status_code": types.Int64Type,
		"headers": types.MapType{ElemType: types.StringType}, "text": types.StringType, "attached_file_id": types.StringType,
	}
}

func CustomResponseAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"uuid": schema.StringAttribute{
			Computed: true, MarkdownDescription: "Custom response UUID, preserved by response name.",
		},
		"name": schema.StringAttribute{
			Required: true, MarkdownDescription: "Unique custom response name within this service.",
			Validators: []validator.String{
				stringvalidator.UTF8LengthBetween(1, 255),
				stringvalidator.RegexMatches(regexp.MustCompile(`\S`), "must contain non-whitespace characters"),
			},
		},
		"status_code": schema.Int64Attribute{
			Required: true, MarkdownDescription: "HTTP response status code (100-599). Redirects 301, 302, 303, 307, and 308 require a non-empty Location header.",
			Validators: []validator.Int64{int64validator.Between(100, 599)},
		},
		"headers": schema.MapAttribute{
			Optional: true, ElementType: types.StringType,
			MarkdownDescription: "Response header names and values. Omit or set {} for no headers.",
		},
		"text": schema.StringAttribute{
			Optional: true, MarkdownDescription: "Inline response body, at most 1024 UTF-8 bytes. Conflicts with attached_file_id. Omit both for an empty body.",
			Validators: []validator.String{
				stringvalidator.LengthAtMost(1024),
				stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("attached_file_id")),
			},
		},
		"attached_file_id": schema.StringAttribute{
			Optional: true, MarkdownDescription: "Attachment UUID, typically ioriver_attached_file.example.id. Conflicts with text.",
			Validators: []validator.String{
				stringvalidator.LengthAtLeast(1),
				stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("text")),
			},
		},
	}
}

func customResponsesToMap(ctx context.Context, values types.List) ([]interface{}, error) {
	result := make([]interface{}, 0)
	if values.IsUnknown() {
		return nil, fmt.Errorf("custom responses are unknown at apply time")
	}
	if values.IsNull() {
		return result, nil
	}
	models, err := ListElementsAs[CustomResponseModel](ctx, values)
	if err != nil {
		return nil, fmt.Errorf("decode custom responses: %w", err)
	}
	names := make(map[string]bool, len(models))
	for _, model := range models {
		if model.Name.IsUnknown() || model.StatusCode.IsUnknown() || model.Headers.IsUnknown() ||
			model.Text.IsUnknown() || model.AttachedFileID.IsUnknown() {
			return nil, fmt.Errorf("custom response contains unknown values at apply time")
		}
		name := model.Name.ValueString()
		if err := validateCustomResponse(ctx, model); err != nil {
			return nil, fmt.Errorf("custom response %q: %w", name, err)
		}
		if names[name] {
			return nil, fmt.Errorf("duplicate custom response name %q", name)
		}
		names[name] = true
		if !model.Text.IsNull() && !model.AttachedFileID.IsNull() {
			return nil, fmt.Errorf("custom response %q cannot set both text and attached_file_id", name)
		}
		id := model.UUID.ValueString()
		if id == "" {
			id = GenerateUUID()
		}
		var headers map[string]string
		if !model.Headers.IsNull() {
			if diags := model.Headers.ElementsAs(ctx, &headers, false); diags.HasError() {
				return nil, fmt.Errorf("decode headers for custom response %q: %v", name, diags)
			}
		}
		headerNames := make([]string, 0, len(headers))
		for header := range headers {
			headerNames = append(headerNames, header)
		}
		sort.Strings(headerNames)
		headerList := make([]map[string]string, 0, len(headers))
		for _, header := range headerNames {
			headerList = append(headerList, map[string]string{"name": header, "value": headers[header]})
		}
		response := map[string]interface{}{
			"uuid": id, "name": name, "status_code": model.StatusCode.ValueInt64(), "headers": headerList,
		}
		if !model.Text.IsNull() {
			response["text"] = model.Text.ValueString()
		}
		if !model.AttachedFileID.IsNull() {
			response["attached_file_id"] = model.AttachedFileID.ValueString()
		}
		result = append(result, response)
	}
	return result, nil
}

func customResponsesFromMap(ctx context.Context, raw interface{}, prior *ServiceConfigModel) (types.List, error) {
	elemType := types.ObjectType{AttrTypes: CustomResponseAttrTypes()}
	null := types.ListNull(elemType)
	priorByName := make(map[string]CustomResponseModel)
	explicit := prior != nil && !prior.CustomResponses.IsNull() && !prior.CustomResponses.IsUnknown()
	if explicit {
		models, err := ListElementsAs[CustomResponseModel](ctx, prior.CustomResponses)
		if err != nil {
			return null, fmt.Errorf("decode prior custom responses: %w", err)
		}
		for _, model := range models {
			priorByName[model.Name.ValueString()] = model
		}
	}
	var entries []interface{}
	if raw != nil {
		var ok bool
		entries, ok = raw.([]interface{})
		if !ok {
			return null, fmt.Errorf("invalid custom responses payload %T", raw)
		}
	}
	if len(entries) == 0 && !explicit {
		return null, nil
	}
	models := make([]CustomResponseModel, 0, len(entries))
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]interface{})
		if !ok {
			return null, fmt.Errorf("invalid custom response entry %T", rawEntry)
		}
		id, idOK := entry["uuid"].(string)
		name, nameOK := entry["name"].(string)
		code, codeOK := entry["status_code"].(float64)
		if !idOK || id == "" || !nameOK || name == "" || !codeOK || code < 100 || code > 999 || code != float64(int64(code)) {
			return null, fmt.Errorf("custom response has invalid uuid, name, or status_code")
		}
		model := CustomResponseModel{
			UUID: types.StringValue(id), Name: types.StringValue(name), StatusCode: types.Int64Value(int64(code)),
			Headers: types.MapNull(types.StringType), Text: types.StringNull(), AttachedFileID: types.StringNull(),
		}
		headers, ok := entry["headers"].([]interface{})
		if !ok {
			return null, fmt.Errorf("invalid headers for custom response %q", name)
		}
		headerValues := make(map[string]string, len(headers))
		for _, rawHeader := range headers {
			header, ok := rawHeader.(map[string]interface{})
			if !ok {
				return null, fmt.Errorf("invalid header entry for custom response %q", name)
			}
			headerName, nameOK := header["name"].(string)
			value, valueOK := header["value"].(string)
			if !nameOK || !valueOK {
				return null, fmt.Errorf("invalid header name or value for custom response %q", name)
			}
			if _, duplicate := headerValues[headerName]; duplicate {
				return null, fmt.Errorf("custom response %q has duplicate header %q, which cannot be represented by a Terraform map", name, headerName)
			}
			headerValues[headerName] = value
		}
		priorModel, hasPrior := priorByName[name]
		if len(headerValues) > 0 || !hasPrior || !priorModel.Headers.IsNull() {
			headersValue, diagnostics := types.MapValueFrom(ctx, types.StringType, headerValues)
			if diagnostics.HasError() {
				return null, fmt.Errorf("encode custom response headers: %v", diagnostics)
			}
			model.Headers = headersValue
		}
		if rawText := entry["text"]; rawText != nil {
			text, ok := rawText.(string)
			if !ok {
				return null, fmt.Errorf("invalid text for custom response %q", name)
			}
			model.Text = types.StringValue(text)
			if hasPrior && !priorModel.Text.IsNull() && !priorModel.Text.IsUnknown() &&
				strings.TrimSpace(priorModel.Text.ValueString()) == text {
				model.Text = priorModel.Text
			}
		}
		if rawID := entry["attached_file_id"]; rawID != nil {
			fileID, ok := rawID.(string)
			if !ok || fileID == "" {
				return null, fmt.Errorf("invalid attachment ID for custom response %q", name)
			}
			model.AttachedFileID = types.StringValue(fileID)
		}
		if !model.Text.IsNull() && !model.AttachedFileID.IsNull() {
			return null, fmt.Errorf("custom response %q has both text and an attachment ID", name)
		}
		models = append(models, model)
	}
	return ListObjectValueFrom(ctx, CustomResponseAttrTypes(), models)
}
