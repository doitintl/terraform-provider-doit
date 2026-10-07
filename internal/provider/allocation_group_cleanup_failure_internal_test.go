package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/doitintl/terraform-provider-doit/internal/provider/models"
	"github.com/doitintl/terraform-provider-doit/internal/provider/resource_allocation"
	"github.com/oapi-codegen/nullable"
)

// inlineRule builds a state/plan rule with the given action and id and one fixed component.
func inlineRuleForTest(t *testing.T, action, id, name string) attr.Value {
	t.Helper()
	return modifyPlanTestRule(t, map[string]attr.Value{
		"action":      types.StringValue(action),
		"id":          types.StringValue(id),
		"name":        types.StringValue(name),
		"description": types.StringNull(),
		"formula":     types.StringValue("A"),
		"components": modifyPlanTestComponentList(t,
			modifyPlanTestComponent(t, "country", []string{"JP"})),
	})
}

func allocationRawForTest(t *testing.T, sch schema.Schema, model allocationResourceModel) tfsdk.State {
	t.Helper()
	ctx := t.Context()
	obj, diags := types.ObjectValueFrom(ctx, sch.Type().(basetypes.ObjectType).AttrTypes, model)
	if diags.HasError() {
		t.Fatalf("object from model: %v", diags)
	}
	raw, err := obj.ToTerraformValue(ctx)
	if err != nil {
		t.Fatalf("to terraform value: %v", err)
	}
	return tfsdk.State{Schema: sch, Raw: raw}
}

// failingMemberServer mimics the API for a group "group-123" whose inline member "member-1" can't
// be deleted (it is referenced elsewhere). The group itself is gone (404) once deleted.
type failingMemberServer struct {
	mu           sync.Mutex
	groupDeleted bool
	deletes      []string
}

func (f *failingMemberServer) handler(patchBody string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/group-123"):
			f.deletes = append(f.deletes, "group-123")
			f.groupDeleted = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete:
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			f.deletes = append(f.deletes, id)
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprintf(w, `[{"id":%q,"type":"single","error":"allocation single is in use"}]`, id)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/group-123") && f.groupDeleted:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/keep-1"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"keep-1","name":"k1","type":"custom","rule":{"formula":"A","components":[{"dimension":{"id":"country","type":"preset"},"mode":"include","values":["JP"]}]}}`))
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(patchBody))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
}

// A member whose deletion fails must not turn into an unrecoverable orphan. An error would leave
// the old state in place, but the next refresh finds the group gone (404) and drops the resource
// together with the member ids, so the cleanup could never be retried. The provider reports the
// leftover ids in a warning instead and completes the delete.
func TestAllocationDelete_MemberCleanupFailureWarns(t *testing.T) {
	ctx := t.Context()
	f := &failingMemberServer{}
	server := httptest.NewTestServer(t, f.handler(""))
	r := &allocationResource{client: newTestAPIClient(t, server)}

	sch := modifyPlanTestSchema(t)
	state := allocationRawForTest(t, sch, modifyPlanTestModel(t,
		[]attr.Value{inlineRuleForTest(t, "create", "member-1", "m1")},
		modifyPlanTestTimeouts(t, sch)))

	resp := &resource.DeleteResponse{}
	r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete must not fail after the group is gone; got: %v", resp.Diagnostics)
	}
	var warned bool
	for _, d := range resp.Diagnostics.Warnings() {
		if strings.Contains(d.Detail(), "member-1") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("expected a warning naming the leftover allocation member-1, got: %v", resp.Diagnostics)
	}
}

// Premise behind the test above: once the group is deleted, Read treats it as externally deleted
// and removes it from state, so a later destroy never reaches Delete again.
func TestAllocationRead_DeletedGroupDropsStateAndMemberIDs(t *testing.T) {
	ctx := t.Context()
	f := &failingMemberServer{groupDeleted: true}
	server := httptest.NewTestServer(t, f.handler(""))
	r := &allocationResource{client: newTestAPIClient(t, server)}

	sch := modifyPlanTestSchema(t)
	state := allocationRawForTest(t, sch, modifyPlanTestModel(t,
		[]attr.Value{inlineRuleForTest(t, "create", "member-1", "m1")},
		modifyPlanTestTimeouts(t, sch)))

	resp := &resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, resp)
	if !resp.State.Raw.IsNull() {
		t.Fatalf("expected Read to remove the deleted group from state")
	}
}

// Same hazard on update: the PATCH already dropped the rule, so after a failed cleanup the next
// refresh no longer lists it and no further plan would retry the delete. Update warns with the
// leftover id and still saves the new state.
func TestAllocationUpdate_RemovedMemberCleanupFailureWarns(t *testing.T) {
	ctx := t.Context()
	f := &failingMemberServer{}
	patchBody := `{"id":"group-123","name":"group-alloc","description":"desc","allocationType":"group","type":"custom",` +
		`"unallocatedCosts":"other","createTime":100,"updateTime":300,"folderId":"root",` +
		`"rules":[{"id":"keep-1","name":"k1","description":""}]}`
	server := httptest.NewTestServer(t, f.handler(patchBody))
	r := &allocationResource{client: newTestAPIClient(t, server)}

	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)
	state := allocationRawForTest(t, sch, modifyPlanTestModel(t, []attr.Value{
		inlineRuleForTest(t, "create", "keep-1", "k1"),
		inlineRuleForTest(t, "create", "member-1", "m1"),
	}, tv))
	planModel := modifyPlanTestModel(t, []attr.Value{inlineRuleForTest(t, "create", "keep-1", "k1")}, tv)
	planModel.UnallocatedCosts = types.StringValue("other")
	plan := allocationRawForTest(t, sch, planModel)
	plan2 := tfsdk.Plan{Schema: sch, Raw: plan.Raw}

	resp := &resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{State: state, Plan: plan2}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update must not fail after the PATCH succeeded; got: %v", resp.Diagnostics)
	}
	var warned bool
	for _, d := range resp.Diagnostics.Warnings() {
		if strings.Contains(d.Detail(), "member-1") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("expected a warning naming the leftover allocation member-1, got: %v", resp.Diagnostics)
	}
	var got allocationResourceModel
	if d := resp.State.Get(ctx, &got); d.HasError() {
		t.Fatalf("reading saved state: %v", d)
	}
	var rules []resource_allocation.RulesValue
	if d := got.Rules.ElementsAs(ctx, &rules, true); d.HasError() || len(rules) != 1 {
		t.Errorf("expected the new state to hold the 1 remaining rule, got %d (%v)", len(rules), d)
	}
}

func TestAllocationUpdate_RemovedMemberCleanedUpAfterChildReadFailure(t *testing.T) {
	ctx := t.Context()
	var deleted []string
	server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPatch:
			_, _ = w.Write([]byte(`{"id":"group-123","name":"group-alloc","description":"desc","allocationType":"group","type":"custom","unallocatedCosts":"other","createTime":100,"updateTime":300,"folderId":"root","rules":[{"id":"keep-1","name":"k1"}]}`))
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodDelete:
			deleted = append(deleted, r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:])
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	r := &allocationResource{client: newTestAPIClient(t, server)}
	sch := modifyPlanTestSchema(t)
	tv := modifyPlanTestTimeouts(t, sch)
	state := allocationRawForTest(t, sch, modifyPlanTestModel(t, []attr.Value{
		inlineRuleForTest(t, "create", "keep-1", "k1"), inlineRuleForTest(t, "create", "removed-1", "removed"),
	}, tv))
	planModel := modifyPlanTestModel(t, []attr.Value{inlineRuleForTest(t, "create", "keep-1", "k1")}, tv)
	planModel.UnallocatedCosts = types.StringValue("other")
	planState := allocationRawForTest(t, sch, planModel)
	resp := &resource.UpdateResponse{State: state}
	r.Update(ctx, resource.UpdateRequest{State: state, Plan: tfsdk.Plan{Schema: sch, Raw: planState.Raw}}, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected child detail error")
	}
	if len(deleted) != 1 || deleted[0] != "removed-1" {
		t.Errorf("deleted = %v, want [removed-1]", deleted)
	}
	var got allocationResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() {
		t.Fatal(diags)
	}
	if got.Id.ValueString() != "group-123" {
		t.Errorf("group ID = %q", got.Id.ValueString())
	}
	var rules []resource_allocation.RulesValue
	if diags := got.Rules.ElementsAs(ctx, &rules, false); diags.HasError() || len(rules) != 1 || rules[0].Id.ValueString() != "keep-1" {
		t.Errorf("recoverable rules = %v (%v)", rules, diags)
	}
}

func TestRemovedInlineRuleIDsFromResponse_RequiresMemberIDs(t *testing.T) {
	prior := []resource_allocation.RulesValue{allocationRuleWithMetadataForTest(t, "owned-1", "prior")}
	rules := []nullable.Nullable[models.GroupAllocationRule]{
		valueToNullable(models.GroupAllocationRule{Name: new("member-without-id")}),
	}
	removed, confirmed := removedInlineRuleIDsFromResponse(prior, &models.Allocation{Rules: &rules})
	if confirmed || len(removed) != 0 {
		t.Errorf("incomplete membership must not authorize deletion: confirmed=%t, removed=%v", confirmed, removed)
	}
}
