---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "awsngfw_prefix_list Data Source - awsngfw"
subcategory: ""
description: |-
  Data source for retrieving prefix list information.
---

# awsngfw_prefix_list (Data Source)

Data source for retrieving prefix list information.

## Example Usage

```terraform
data "awsngfw_prefix_list" "example" {
  rulestack = awsngfw_rulestack.r.name
  name      = "foobar"
}

resource "awsngfw_rulestack" "r" {
  name        = "my-rulestack"
  scope       = "Local"
  account_id  = "12345"
  description = "Made by Terraform"
  profile_config {
    anti_spyware = "BestPractice"
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- **name** (String) The name.
- **rulestack** (String) The rulestack.

### Optional

- **config_type** (String) Retrieve either the candidate or running config. Valid values are `candidate` or `running`. Defaults to `candidate`.
- **id** (String) The ID of this resource.

### Read-Only

- **audit_comment** (String) The audit comment.
- **description** (String) The description.
- **prefix_list** (Set of String) The prefix list.
- **update_token** (String) The update token.

