package honeycombio

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	honeycombio "github.com/kvrhdn/go-honeycombio"
)

func newTrigger() *schema.Resource {
	return &schema.Resource{
		Create: resourceTriggerCreate,
		Read:   resourceTriggerRead,
		Update: resourceTriggerUpdate,
		Delete: resourceTriggerDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"dataset": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"disabled": {
				Type:     schema.TypeBool,
				Optional: true,
			},
			"query": createQuerySpecSchema(&querySpecConstraints{
				calculationCount: 1,
			}),
			"threshold": {
				Type:     schema.TypeSet,
				Required: true,
				MinItems: 1,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"op": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice([]string{">", ">=", "<", "<="}, false),
						},
						"value": {
							Type:     schema.TypeFloat,
							Required: true,
						},
					},
				},
			},
			"frequency": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
				ValidateFunc: validation.All(
					validation.IntDivisibleBy(60),
					validation.IntBetween(60, 86400),
				),
			},
			"recipient": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Required: true,
							// TODO there are more valid recipient types
							ValidateFunc: validation.StringInSlice([]string{"email", "pagerduty"}, false),
						},
						"target": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func resourceTriggerCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)
	t := expandTrigger(d)

	t, err := client.Triggers.Create(dataset, t)
	if err != nil {
		return err
	}

	d.SetId(t.ID)
	return resourceTriggerRead(d, meta)
}

func resourceTriggerRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)

	t, err := client.Triggers.Get(dataset, d.Id())
	if err != nil {
		if err == honeycombio.ErrNotFound {
			d.SetId("")
			return nil
		}
		return err
	}

	// API returns nil for filterCombination if set to the default value "AND"
	// To keep the Terraform config simple, we'll explicitly set "AND" ourself
	if t.Query.FilterCombination == nil {
		filterCombination := honeycombio.FilterCombinationAnd
		t.Query.FilterCombination = &filterCombination
	}

	d.SetId(t.ID)
	d.Set("name", t.Name)
	d.Set("description", t.Description)
	d.Set("disabled", t.Disabled)

	// TODO the diffs don't work well - it wants to replace the entire query block
	err = d.Set("query", flattenQuerySpec(t.Query))
	if err != nil {
		return err
	}

	// TODO the diffs don't work well - it wants to replace the entire threshold block
	err = d.Set("threshold", flattenTriggerThreshold(t.Threshold))
	if err != nil {
		return err
	}

	d.Set("frequency", t.Frequency)

	err = d.Set("recipient", flattenTriggerRecipients(t.Recipients))
	if err != nil {
		return err
	}

	return nil
}

func resourceTriggerUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)
	t := expandTrigger(d)

	t, err := client.Triggers.Update(dataset, t)
	if err != nil {
		return err
	}

	d.SetId(t.ID)
	return resourceTriggerRead(d, meta)
}

func resourceTriggerDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*honeycombio.Client)

	dataset := d.Get("dataset").(string)

	return client.Triggers.Delete(dataset, d.Id())
}

func expandTrigger(d *schema.ResourceData) *honeycombio.Trigger {
	trigger := &honeycombio.Trigger{
		ID:          d.Id(),
		Name:        d.Get("name").(string),
		Description: d.Get("description").(string),
		Disabled:    d.Get("disabled").(bool),
		Query:       expandQuerySpec(d.Get("query").(*schema.Set)),
		Threshold:   expandTriggerThreshold(d.Get("threshold").(*schema.Set)),
		Frequency:   d.Get("frequency").(int),
		Recipients:  expandTriggerRecipients(d.Get("recipient").(*schema.Set)),
	}
	return trigger
}

func expandTriggerThreshold(s *schema.Set) *honeycombio.TriggerThreshold {
	d := s.List()[0].(map[string]interface{})

	value := d["value"].(float64)

	return &honeycombio.TriggerThreshold{
		Op:    honeycombio.TriggerThresholdOp(d["op"].(string)),
		Value: &value,
	}
}

func expandTriggerRecipients(s *schema.Set) []honeycombio.TriggerRecipient {
	rs := s.List()
	triggerRecipients := make([]honeycombio.TriggerRecipient, len(rs))

	for i, r := range rs {
		rMap := r.(map[string]interface{})

		triggerRecipients[i] = honeycombio.TriggerRecipient{
			Type:   rMap["type"].(string),
			Target: rMap["target"].(string),
		}
	}

	return triggerRecipients
}

func flattenTriggerThreshold(t *honeycombio.TriggerThreshold) []map[string]interface{} {
	return []map[string]interface{}{
		{
			"op":    t.Op,
			"value": t.Value,
		},
	}
}

func flattenTriggerRecipients(rs []honeycombio.TriggerRecipient) []map[string]interface{} {
	result := make([]map[string]interface{}, len(rs))

	for i, r := range rs {
		result[i] = map[string]interface{}{
			"type":   r.Type,
			"target": r.Target,
		}
	}

	return result
}
