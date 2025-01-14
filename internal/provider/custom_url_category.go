package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/paloaltonetworks/cloud-ngfw-aws-go"
	"github.com/paloaltonetworks/cloud-ngfw-aws-go/object/url"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// Data source.
func dataSourceCustomUrlCategory() *schema.Resource {
	return &schema.Resource{
		Description: "Data source for retrieving custom url category information.",

		ReadContext: readCustomUrlCategoryDataSource,

		Schema: customUrlCategorySchema(false, nil),
	}
}

func readCustomUrlCategoryDataSource(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := url.NewClient(meta.(*awsngfw.Client))

	style := d.Get(ConfigTypeName).(string)
	d.Set(ConfigTypeName, style)

	stack := d.Get(RulestackName).(string)
	name := d.Get("name").(string)

	id := configTypeId(style, buildCustomUrlCategoryId(stack, name))

	req := url.ReadInput{
		Rulestack: stack,
		Name:      name,
	}
	switch style {
	case CandidateConfig:
		req.Candidate = true
	case RunningConfig:
		req.Running = true
	}

	tflog.Info(
		ctx, "read custom url category",
		"ds", true,
		ConfigTypeName, style,
		RulestackName, req.Rulestack,
		"name", req.Name,
	)

	res, err := svc.Read(ctx, req)
	if err != nil {
		if isObjectNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.SetId(id)

	var info *url.Info
	switch style {
	case CandidateConfig:
		info = res.Response.Candidate
	case RunningConfig:
		info = res.Response.Running
	}
	saveCustomUrlCategory(d, stack, name, *info)

	return nil
}

// Resource.
func resourceCustomUrlCategory() *schema.Resource {
	return &schema.Resource{
		Description: "Resource for custom url category manipulation.",

		CreateContext: createCustomUrlCategory,
		ReadContext:   readCustomUrlCategory,
		UpdateContext: updateCustomUrlCategory,
		DeleteContext: deleteCustomUrlCategory,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: customUrlCategorySchema(true, []string{ConfigTypeName}),
	}
}

func createCustomUrlCategory(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := url.NewClient(meta.(*awsngfw.Client))
	o := loadCustomUrlCategory(d)
	tflog.Info(
		ctx, "create custom url category",
		RulestackName, o.Rulestack,
		"name", o.Name,
	)

	if err := svc.Create(ctx, o); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(buildCustomUrlCategoryId(o.Rulestack, o.Name))

	return readCustomUrlCategory(ctx, d, meta)
}

func readCustomUrlCategory(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := url.NewClient(meta.(*awsngfw.Client))
	stack, name, err := parseCustomUrlCategoryId(d.Id())
	if err != nil {
		return diag.Errorf("Error in parsing ID %q: %s", d.Id(), err)
	}

	req := url.ReadInput{
		Rulestack: stack,
		Name:      name,
		Candidate: true,
	}
	tflog.Info(
		ctx, "read custom url category",
		RulestackName, req.Rulestack,
		"name", name,
	)

	res, err := svc.Read(ctx, req)
	if err != nil {
		if isObjectNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	saveCustomUrlCategory(d, stack, name, *res.Response.Candidate)

	return nil
}

func updateCustomUrlCategory(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := url.NewClient(meta.(*awsngfw.Client))
	o := loadCustomUrlCategory(d)
	tflog.Info(
		ctx, "update custom url category",
		RulestackName, o.Rulestack,
		"name", o.Name,
	)

	if err := svc.Update(ctx, o); err != nil {
		return diag.FromErr(err)
	}

	return readCustomUrlCategory(ctx, d, meta)
}

func deleteCustomUrlCategory(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := url.NewClient(meta.(*awsngfw.Client))
	stack, name, err := parseCustomUrlCategoryId(d.Id())
	if err != nil {
		return diag.Errorf("Error in parsing ID %q: %s", d.Id(), err)
	}

	tflog.Info(
		ctx, "delete custom url category",
		RulestackName, stack,
		"name", name,
	)

	if err := svc.Delete(ctx, stack, name); err != nil && !isObjectNotFound(err) {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

// Schema handling.
func customUrlCategorySchema(isResource bool, rmKeys []string) map[string]*schema.Schema {
	action_values := []string{"none", "alert", "allow", "block", "continue", "override"}

	ans := map[string]*schema.Schema{
		ConfigTypeName: configTypeSchema(),
		RulestackName:  rsSchema(),
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name.",
			ForceNew:    true,
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The description.",
		},
		"url_list": {
			Type:        schema.TypeSet,
			Required:    true,
			Description: "The URL list for this custom URL category.",
			MinItems:    1,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
		"action": {
			Type:         schema.TypeString,
			Optional:     true,
			Description:  addStringInSliceValidation("The action to take.", action_values),
			Default:      "none",
			ValidateFunc: validation.StringInSlice(action_values, false),
		},
		"audit_comment": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The audit comment.",
		},
		"update_token": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The update token.",
		},
	}

	for _, rmKey := range rmKeys {
		delete(ans, rmKey)
	}

	if !isResource {
		computed(ans, "", []string{ConfigTypeName, RulestackName, "name"})
	}

	return ans
}

func loadCustomUrlCategory(d *schema.ResourceData) url.Info {
	return url.Info{
		Rulestack:    d.Get(RulestackName).(string),
		Name:         d.Get("name").(string),
		Description:  d.Get("description").(string),
		UrlList:      setToSlice(d.Get("url_list")),
		Action:       d.Get("action").(string),
		AuditComment: d.Get("audit_comment").(string),
	}
}

func saveCustomUrlCategory(d *schema.ResourceData, stack, name string, o url.Info) {
	d.Set(RulestackName, stack)
	d.Set("name", name)
	d.Set("description", o.Description)
	d.Set("url_list", sliceToSet(o.UrlList))
	d.Set("action", o.Action)
	d.Set("audit_comment", o.AuditComment)
	d.Set("update_token", o.UpdateToken)
}

// Id functions.
func buildCustomUrlCategoryId(a, b string) string {
	return strings.Join([]string{a, b}, IdSeparator)
}

func parseCustomUrlCategoryId(v string) (string, string, error) {
	tok := strings.Split(v, IdSeparator)
	if len(tok) != 2 {
		return "", "", fmt.Errorf("Expecting 2 tokens, got %d", len(tok))
	}

	return tok[0], tok[1], nil
}
