package provider

import (
	"context"

	"github.com/paloaltonetworks/cloud-ngfw-aws-go"
	"github.com/paloaltonetworks/cloud-ngfw-aws-go/firewall"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Data source for a single firewall.
func dataSourceFirewall() *schema.Resource {
	return &schema.Resource{
		Description: "Data source for retrieving firewall information.",

		ReadContext: readFirewallDataSource,

		Schema: firewallSchema(false, nil),
	}
}

func readFirewallDataSource(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := firewall.NewClient(meta.(*awsngfw.Client))
	region := meta.(*awsngfw.Client).Region

	name := d.Get("name").(string)
	account_id := d.Get("account_id").(string)

	req := firewall.ReadInput{
		Name:      name,
		AccountId: account_id,
	}

	tflog.Info(
		ctx, "read firewall",
		"ds", true,
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

	account_id = res.Response.Firewall.AccountId
	id := firewallId(account_id, region, name)
	d.SetId(id)

	saveFirewall(ctx, d, name, *res.Response)

	return nil
}

// Resource.
func resourceFirewall() *schema.Resource {
	return &schema.Resource{
		Description: "Resource for firewall manipulation.",

		CreateContext: createFirewall,
		ReadContext:   readFirewall,
		UpdateContext: updateFirewall,
		DeleteContext: deleteFirewall,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: firewallSchema(true, []string{"status", "endpoint_service_name"}),
	}
}

func createFirewall(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := firewall.NewClient(meta.(*awsngfw.Client))
	region := meta.(*awsngfw.Client).Region
	o := loadFirewall(ctx, d)

	tflog.Info(
		ctx, "create firewall",
		"name", o.Name,
		"payload", o,
	)

	var err error
	var res firewall.ReadOutput
	if res, err = svc.Create(ctx, o); err != nil {
		return diag.FromErr(err)
	}

	fw := res.Response.Firewall
	id := firewallId(fw.AccountId, region, fw.Name)
	d.SetId(id)

	return readFirewall(ctx, d, meta)
}

func readFirewall(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := firewall.NewClient(meta.(*awsngfw.Client))

	name := d.Get("name").(string)
	account_id := d.Get("account_id").(string)

	req := firewall.ReadInput{
		Name:      name,
		AccountId: account_id,
	}

	tflog.Info(
		ctx, "read firewall",
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

	saveFirewall(ctx, d, name, *res.Response)

	return nil
}

func updateFirewall(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := firewall.NewClient(meta.(*awsngfw.Client))

	o := loadFirewall(ctx, d)

	tflog.Info(
		ctx, "update firewall",
		"name", o.Name,
	)

	if d.HasChange("description") {
		if err := svc.UpdateDescription(ctx, o); err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("app_id_version") && d.Get("app_id_version") != "" {
		if err := svc.UpdateNGFirewallContentVersion(ctx, o); err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("subnet_mapping") {

		old_mapping, new_mapping := d.GetChange("subnet_mapping")
		o_subnets, n_subnets := []string{}, []string{}

		for _, s := range old_mapping.([]interface{}) {
			mapping := s.(map[string]interface{})
			o_subnets = append(o_subnets, mapping["subnet_id"].(string))
		}
		for _, s := range new_mapping.([]interface{}) {
			mapping := s.(map[string]interface{})
			n_subnets = append(n_subnets, mapping["subnet_id"].(string))
		}

		assoc := []firewall.SubnetMapping{}
		dassoc := []firewall.SubnetMapping{}

		for _, i := range sliceDiff(n_subnets, o_subnets) {
			m := firewall.SubnetMapping{
				SubnetId: i,
			}
			assoc = append(assoc, m)
		}

		for _, i := range sliceDiff(o_subnets, n_subnets) {
			m := firewall.SubnetMapping{
				SubnetId: i,
			}
			dassoc = append(dassoc, m)
		}

		o.AssociateSubnetMappings = assoc
		o.DisassociateSubnetMappings = dassoc
		if err := svc.UpdateSubnetMappings(ctx, o); err != nil {
			return diag.FromErr(err)
		}
	}

	return readFirewall(ctx, d, meta)
}

func deleteFirewall(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	svc := firewall.NewClient(meta.(*awsngfw.Client))
	name := d.Get("name").(string)
	var account_id string
	if acc_id, ok := d.GetOk("account_id"); ok {
		account_id = acc_id.(string)
	}

	tflog.Info(
		ctx, "delete firewall",
		"name", name,
	)

	fw := firewall.ReadInput{
		Name:      name,
		AccountId: account_id,
	}

	if err := svc.Delete(ctx, fw); err != nil && !isObjectNotFound(err) {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

// Schema handling.
func firewallSchema(isResource bool, rmKeys []string) map[string]*schema.Schema {
	ans := map[string]*schema.Schema{
		"name": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The name.",
			ForceNew:    true,
		},
		"vpc_id": {
			Type:        schema.TypeString,
			Required:    true,
			Description: "The vpc id.",
			ForceNew:    true,
		},
		"account_id": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The account ID.",
			ForceNew:    true,
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "The description.",
		},
		"endpoint_mode": endpointModeSchema(),
		"endpoint_service_name": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The endpoint service name.",
		},
		"subnet_mapping": {
			Type:        schema.TypeList,
			Required:    true,
			Description: "Subnet mappings.",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"subnet_id": {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "The subnet id.",
					},
					"az": {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "The availability zone.",
					},
					// Future use
					// "az_id": {
					// 	Type:        schema.TypeString,
					// 	Optional:    true,
					// 	Description: "The availability zone id.",
					// },
				},
			},
		},
		"app_id_version": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "App-ID version number.",
		},
		"automatic_upgrade_app_id_version": {
			Type:        schema.TypeBool,
			Optional:    true,
			Description: "Automatic App-ID upgrade version number.",
			Default:     true,
		},
		RulestackName:       rsSchema(),
		GlobalRulestackName: gRsSchema(),
		"tags": {
			Type:        schema.TypeMap,
			Optional:    true,
			Description: "Tags.",
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
		"update_token": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "The update token.",
		},
		"status": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"firewall_status": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "The firewall status.",
					},
					"failure_reason": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "The firewall failure reason.",
					},
					"rulestack_status": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "The rulestack status.",
					},
					"attachments": {
						Type:        schema.TypeList,
						Computed:    true,
						Description: "The firewall attachments.",
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"endpoint_id": {
									Type:        schema.TypeString,
									Computed:    true,
									Description: "The endpoint id.",
								},
								"status": {
									Type:        schema.TypeString,
									Computed:    true,
									Description: "The attachment status.",
								},
								"rejected_reason": {
									Type:        schema.TypeString,
									Computed:    true,
									Description: "The reject reason.",
								},
								"subnet_id": {
									Type:        schema.TypeString,
									Computed:    true,
									Description: "The subnet id.",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, rmKey := range rmKeys {
		delete(ans, rmKey)
	}

	if !isResource {
		computed(ans, "", []string{"name"})
	}

	return ans
}

func loadFirewall(ctx context.Context, d *schema.ResourceData) firewall.Info {

	return firewall.Info{
		Name:                         d.Get("name").(string),
		VpcId:                        d.Get("vpc_id").(string),
		AccountId:                    d.Get("account_id").(string),
		Description:                  d.Get("description").(string),
		EndpointMode:                 d.Get("endpoint_mode").(string),
		SubnetMappings:               loadSubnetMappings(ctx, d.Get("subnet_mapping").([]interface{})),
		AppIdVersion:                 d.Get("app_id_version").(string),
		AutomaticUpgradeAppIdVersion: d.Get("automatic_upgrade_app_id_version").(bool),
		RuleStackName:                d.Get(RulestackName).(string),
		GlobalRuleStackName:          d.Get(GlobalRulestackName).(string),
		Tags:                         loadTags(ctx, d.Get("tags").(map[string]interface{})),
	}
}

func saveFirewall(ctx context.Context, d *schema.ResourceData, name string, o firewall.ReadResponse) {
	d.Set("name", name)
	d.Set("vpc_id", o.Firewall.VpcId)
	d.Set("account_id", o.Firewall.AccountId)
	d.Set("description", o.Firewall.Description)
	d.Set("endpoint_mode", o.Firewall.EndpointMode)
	d.Set("endpoint_service_name", o.Firewall.EndpointServiceName)
	d.Set("subnet_mapping", saveSubnetMappings(ctx, o.Firewall.SubnetMappings))
	d.Set("app_id_version", o.Firewall.AppIdVersion)
	d.Set("automatic_upgrade_app_id_version", o.Firewall.AutomaticUpgradeAppIdVersion)
	d.Set(RulestackName, o.Firewall.RuleStackName)
	d.Set(GlobalRulestackName, o.Firewall.GlobalRuleStackName)
	d.Set("tags", saveTags(ctx, o.Firewall.Tags))
	d.Set("update_token", o.Firewall.UpdateToken)
	if o.Status != nil {
		d.Set("status", saveStatus(ctx, *o.Status))
	}

}

func saveSubnetMappings(ctx context.Context, subnetMappings []firewall.SubnetMapping) []interface{} {
	if subnetMappings != nil {
		mappings := make([]interface{}, len(subnetMappings), len(subnetMappings))

		for i, sm := range subnetMappings {
			_sm := make(map[string]interface{})
			_sm["subnet_id"] = sm.SubnetId
			_sm["az"] = sm.AvailabilityZone
			// _sm["az_id"] = sm.AvailabilityZoneId
			mappings[i] = _sm
		}
		return mappings
	}

	return make([]interface{}, 0)
}

func loadSubnetMappings(ctx context.Context, subnetMappings []interface{}) []firewall.SubnetMapping {
	if subnetMappings != nil {
		mappings := make([]firewall.SubnetMapping, len(subnetMappings), len(subnetMappings))

		for i, sm := range subnetMappings {
			_smi := sm.(map[string]interface{})
			_sm := firewall.SubnetMapping{
				SubnetId:         _smi["subnet_id"].(string),
				AvailabilityZone: _smi["az"].(string),
			}
			mappings[i] = _sm
		}

		return mappings
	}

	return make([]firewall.SubnetMapping, 0)
}

func saveTags(ctx context.Context, tags []firewall.TagDetails) map[string]interface{} {
	if tags != nil {
		t := make(map[string]interface{})
		for _, v := range tags {
			t[v.Key] = v.Value
		}
		return t
	}

	return make(map[string]interface{})
}

func loadTags(ctx context.Context, tags map[string]interface{}) []firewall.TagDetails {
	if tags != nil {
		_tags := []firewall.TagDetails{}

		for k, v := range tags {
			t := firewall.TagDetails{
				Key:   k,
				Value: v.(string),
			}
			_tags = append(_tags, t)
		}

		return _tags
	}

	return make([]firewall.TagDetails, 0)
}

func saveStatus(ctx context.Context, status firewall.FirewallStatus) []interface{} {

	s := make([]interface{}, 1, 1)

	attachments := make([]map[string]interface{}, len(status.Attachments), len(status.Attachments))

	for i, att := range status.Attachments {
		_att := make(map[string]interface{})
		_att["endpoint_id"] = att.EndpointId
		_att["status"] = att.Status
		_att["rejected_reason"] = att.RejectedReason
		_att["subnet_id"] = att.SubnetId
		attachments[i] = _att
	}

	_s := make(map[string]interface{})

	_s["firewall_status"] = status.FirewallStatus
	_s["failure_reason"] = status.FailureReason
	_s["rulestack_status"] = status.RuleStackStatus
	_s["attachments"] = attachments

	s[0] = _s

	return s

}