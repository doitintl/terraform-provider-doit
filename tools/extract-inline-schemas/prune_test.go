package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// --- loadProviderConfigs ---

func TestLoadProviderConfigs(t *testing.T) {
	dir := t.TempDir()

	datasources := filepath.Join(dir, "datasources.yml")
	resources := filepath.Join(dir, "resources.yml")

	dsContent := `
provider:
  name: test
data_sources:
  user:
    read:
      path: /users/{id}
      method: GET
  users:
    read:
      path: /users
      method: GET
`
	resContent := `
provider:
  name: test
resources:
  user:
    create:
      path: /users
      method: POST
    read:
      path: /users/{id}
      method: GET
    update:
      path: /users/{id}
      method: PATCH
    delete:
      path: /users/{id}
      method: DELETE
`

	if err := os.WriteFile(datasources, []byte(dsContent), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resources, []byte(resContent), 0600); err != nil {
		t.Fatal(err)
	}

	used, err := loadProviderConfigs(datasources, resources)
	if err != nil {
		t.Fatalf("loadProviderConfigs: %v", err)
	}

	expected := []pathMethod{
		{"/users/{id}", "get"},
		{"/users", "get"},
		{"/users", "post"},
		{"/users/{id}", "patch"},
		{"/users/{id}", "delete"},
	}

	for _, pm := range expected {
		if !used[pm] {
			t.Errorf("expected %s %s to be used", pm.method, pm.path)
		}
	}

	// Should not contain uppercase or unknown paths
	if used[pathMethod{"/users/{id}", "GET"}] {
		t.Error("methods should be lowercased")
	}
}

func TestLoadProviderConfigs_FileNotFound(t *testing.T) {
	_, err := loadProviderConfigs("/nonexistent/datasources.yml", "/nonexistent/resources.yml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- loadExtraPaths ---

func TestLoadExtraPaths(t *testing.T) {
	dir := t.TempDir()
	extraFile := filepath.Join(dir, "extra_paths.yml")

	content := `
paths:
  - path: /reports/query
    method: POST
  - path: /reports/{id}
    method: GET
`
	if err := os.WriteFile(extraFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	used := make(map[pathMethod]bool)
	if err := loadExtraPaths(extraFile, used); err != nil {
		t.Fatalf("loadExtraPaths: %v", err)
	}

	if !used[pathMethod{"/reports/query", "post"}] {
		t.Error("expected /reports/query POST to be used")
	}
	if !used[pathMethod{"/reports/{id}", "get"}] {
		t.Error("expected /reports/{id} GET to be used")
	}
}

func TestLoadExtraPaths_FileNotFound(t *testing.T) {
	used := make(map[pathMethod]bool)
	err := loadExtraPaths("/nonexistent/extra.yml", used)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// --- prunePaths ---

func TestPrunePaths(t *testing.T) {
	spec := `
/users:
  get:
    operationId: listUsers
  post:
    operationId: createUser
/users/{id}:
  get:
    operationId: getUser
  delete:
    operationId: deleteUser
/admin/stats:
  get:
    operationId: getAdminStats
`
	pathsNode := mustParseYAML(t, spec)

	used := map[pathMethod]bool{
		{"/users", "get"}:      true,
		{"/users/{id}", "get"}: true,
	}

	pathsRemoved, methodsPruned := prunePaths(pathsNode, used)

	// /admin/stats should be removed entirely
	if pathsRemoved != 1 {
		t.Errorf("pathsRemoved = %d, want 1", pathsRemoved)
	}

	// POST /users and DELETE /users/{id} should be pruned
	if methodsPruned != 2 {
		t.Errorf("methodsPruned = %d, want 2", methodsPruned)
	}

	// Verify remaining paths
	if pathsNode.Kind != yaml.MappingNode {
		t.Fatal("expected mapping node")
	}

	// Should have 2 paths remaining (4 content nodes: key+value pairs)
	if len(pathsNode.Content) != 4 {
		t.Errorf("remaining path entries = %d, want 4 (2 paths)", len(pathsNode.Content))
	}

	// Check /users only has GET
	usersPath := getMappingValue(pathsNode, "/users")
	if usersPath == nil {
		t.Fatal("/users path missing")
	}
	if len(usersPath.Content) != 2 { // just "get" key + value
		t.Errorf("/users methods = %d content nodes, want 2", len(usersPath.Content))
	}
}

func TestPrunePaths_NoUsed(t *testing.T) {
	spec := `
/users:
  get:
    operationId: listUsers
`
	pathsNode := mustParseYAML(t, spec)
	used := map[pathMethod]bool{}

	pathsRemoved, _ := prunePaths(pathsNode, used)
	if pathsRemoved != 1 {
		t.Errorf("pathsRemoved = %d, want 1", pathsRemoved)
	}
	if len(pathsNode.Content) != 0 {
		t.Errorf("remaining content = %d, want 0", len(pathsNode.Content))
	}
}

func TestPrunePaths_NonHTTPKeys(t *testing.T) {
	// Paths can have non-HTTP keys like "parameters" at the path level
	spec := `
/users:
  parameters:
    - name: org
      in: query
  get:
    operationId: listUsers
`
	pathsNode := mustParseYAML(t, spec)
	used := map[pathMethod]bool{
		{"/users", "get"}: true,
	}

	pathsRemoved, methodsPruned := prunePaths(pathsNode, used)
	if pathsRemoved != 0 {
		t.Errorf("pathsRemoved = %d, want 0", pathsRemoved)
	}
	if methodsPruned != 0 {
		t.Errorf("methodsPruned = %d, want 0", methodsPruned)
	}

	// Both "parameters" and "get" should remain
	usersPath := getMappingValue(pathsNode, "/users")
	if usersPath == nil {
		t.Fatal("/users path missing")
	}
	if len(usersPath.Content) != 4 { // parameters + get
		t.Errorf("/users content nodes = %d, want 4", len(usersPath.Content))
	}
}

// --- collectReachableSchemas ---

func TestCollectReachableSchemas(t *testing.T) {
	paths := mustParseYAML(t, `
/users:
  get:
    responses:
      "200":
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UserList"
`)

	schemas := mustParseYAML(t, `
UserList:
  type: object
  properties:
    items:
      type: array
      items:
        $ref: "#/components/schemas/User"
User:
  type: object
  properties:
    name:
      type: string
    role:
      $ref: "#/components/schemas/Role"
Role:
  type: object
  properties:
    name:
      type: string
Unused:
  type: object
  properties:
    foo:
      type: string
`)

	reachable := collectReachableSchemas(nil, paths, schemas)

	for _, name := range []string{"UserList", "User", "Role"} {
		if !reachable[name] {
			t.Errorf("expected %q to be reachable", name)
		}
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

func TestCollectReachableSchemas_CircularRef(t *testing.T) {
	paths := mustParseYAML(t, `
/tree:
  get:
    responses:
      "200":
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/TreeNode"
`)

	schemas := mustParseYAML(t, `
TreeNode:
  type: object
  properties:
    children:
      type: array
      items:
        $ref: "#/components/schemas/TreeNode"
`)

	// Should not infinite loop
	reachable := collectReachableSchemas(nil, paths, schemas)
	if !reachable["TreeNode"] {
		t.Error("TreeNode should be reachable")
	}
}

func TestCollectReachableSchemas_AllOfRef(t *testing.T) {
	paths := mustParseYAML(t, `
/items:
  get:
    responses:
      "200":
        content:
          application/json:
            schema:
              allOf:
                - $ref: "#/components/schemas/Base"
                - type: object
                  properties:
                    extra:
                      $ref: "#/components/schemas/Extra"
`)

	schemas := mustParseYAML(t, `
Base:
  type: object
  properties:
    id:
      type: string
Extra:
  type: object
  properties:
    value:
      type: string
Orphan:
  type: object
`)

	reachable := collectReachableSchemas(nil, paths, schemas)

	if !reachable["Base"] {
		t.Error("Base should be reachable via allOf")
	}
	if !reachable["Extra"] {
		t.Error("Extra should be reachable via allOf property")
	}
	if reachable["Orphan"] {
		t.Error("Orphan should not be reachable")
	}
}

func TestCollectReachableSchemas_ResponseRef(t *testing.T) {
	// Simulate a full spec with components/responses that reference schemas
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /users:
    get:
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "400":
          $ref: "#/components/responses/BadRequest"
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
    Error:
      type: object
      properties:
        message:
          type: string
    Unused:
      type: object
  responses:
    BadRequest:
      description: Bad Request
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatal(err)
	}
	doc := root.Content[0]

	pathsNode := findMapValue(doc, "paths")
	schemasNode := findMapValue(doc, "components", "schemas")

	reachable := collectReachableSchemas(doc, pathsNode, schemasNode)

	if !reachable["User"] {
		t.Error("User should be reachable directly")
	}
	if !reachable["Error"] {
		t.Error("Error should be reachable via components/responses/BadRequest")
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

// --- pruneSchemas ---

func TestPruneSchemas(t *testing.T) {
	schemas := mustParseYAML(t, `
Alpha:
  type: object
Beta:
  type: object
Gamma:
  type: object
`)

	reachable := map[string]bool{
		"Alpha": true,
		"Gamma": true,
	}

	removed := pruneSchemas(schemas, reachable)
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// Should have Alpha and Gamma
	if schemas.Kind != yaml.MappingNode {
		t.Fatal("expected mapping node")
	}
	if len(schemas.Content) != 4 { // 2 pairs
		t.Errorf("remaining schema entries = %d content nodes, want 4", len(schemas.Content))
	}

	if getMappingValue(schemas, "Alpha") == nil {
		t.Error("Alpha should remain")
	}
	if getMappingValue(schemas, "Beta") != nil {
		t.Error("Beta should be pruned")
	}
	if getMappingValue(schemas, "Gamma") == nil {
		t.Error("Gamma should remain")
	}
}

func TestPruneSchemas_AllReachable(t *testing.T) {
	schemas := mustParseYAML(t, `
A:
  type: object
B:
  type: object
`)
	reachable := map[string]bool{"A": true, "B": true}
	removed := pruneSchemas(schemas, reachable)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

// --- End-to-end pruning ---

func TestEndToEndPrune(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test API
  version: v1
paths:
  /users:
    get:
      operationId: listUsers
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/User"
    post:
      operationId: createUser
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateUserRequest"
  /admin/stats:
    get:
      operationId: getStats
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/AdminStats"
components:
  schemas:
    User:
      type: object
      properties:
        name:
          type: string
        role:
          $ref: "#/components/schemas/Role"
    Role:
      type: object
      properties:
        name:
          type: string
    CreateUserRequest:
      type: object
      properties:
        name:
          type: string
    AdminStats:
      type: object
      properties:
        count:
          type: integer
    OrphanSchema:
      type: object
      properties:
        unused:
          type: string
`

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatalf("failed to parse spec: %v", err)
	}
	doc := root.Content[0]

	pathsNode := findMapValue(doc, "paths")
	schemasNode := findMapValue(doc, "components", "schemas")

	// Only keep GET /users
	used := map[pathMethod]bool{
		{"/users", "get"}: true,
	}

	pathsRemoved, methodsPruned := prunePaths(pathsNode, used)
	if pathsRemoved != 1 { // /admin/stats removed
		t.Errorf("pathsRemoved = %d, want 1", pathsRemoved)
	}
	if methodsPruned != 1 { // POST /users pruned
		t.Errorf("methodsPruned = %d, want 1", methodsPruned)
	}

	reachable := collectReachableSchemas(doc, pathsNode, schemasNode)

	// User and Role should be reachable (User -> Role via $ref)
	if !reachable["User"] {
		t.Error("User should be reachable")
	}
	if !reachable["Role"] {
		t.Error("Role should be reachable")
	}
	// CreateUserRequest should NOT be reachable (POST /users was pruned)
	if reachable["CreateUserRequest"] {
		t.Error("CreateUserRequest should not be reachable (POST was pruned)")
	}
	// AdminStats should NOT be reachable (/admin/stats was removed)
	if reachable["AdminStats"] {
		t.Error("AdminStats should not be reachable")
	}
	// OrphanSchema should NOT be reachable
	if reachable["OrphanSchema"] {
		t.Error("OrphanSchema should not be reachable")
	}

	removed := pruneSchemas(schemasNode, reachable)
	if removed != 3 { // CreateUserRequest, AdminStats, OrphanSchema
		t.Errorf("schemas removed = %d, want 3", removed)
	}

	// Verify only User and Role remain
	if getMappingValue(schemasNode, "User") == nil {
		t.Error("User should remain")
	}
	if getMappingValue(schemasNode, "Role") == nil {
		t.Error("Role should remain")
	}
}

// --- collectRefsFromNode ---

func TestCollectRefsFromNode(t *testing.T) {
	node := mustParseYAML(t, `
allOf:
  - $ref: "#/components/schemas/Foo"
  - type: object
    properties:
      bar:
        $ref: "#/components/schemas/Bar"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	if len(schemaQueue) != 2 {
		t.Fatalf("schemaQueue length = %d, want 2", len(schemaQueue))
	}

	found := map[string]bool{}
	for _, name := range schemaQueue {
		found[name] = true
	}
	if !found["Foo"] {
		t.Error("expected Foo in schemaQueue")
	}
	if !found["Bar"] {
		t.Error("expected Bar in schemaQueue")
	}
	if len(responseQueue) != 0 {
		t.Errorf("responseQueue should be empty, got %d", len(responseQueue))
	}
}

func TestCollectRefsFromNode_NonSchemaRef(t *testing.T) {
	node := mustParseYAML(t, `
$ref: "#/components/responses/NotFound"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	// Response refs should go to responseQueue
	if len(schemaQueue) != 0 {
		t.Errorf("schemaQueue length = %d, want 0", len(schemaQueue))
	}
	if len(responseQueue) != 1 {
		t.Fatalf("responseQueue length = %d, want 1", len(responseQueue))
	}
	if responseQueue[0] != "NotFound" {
		t.Errorf("responseQueue[0] = %q, want %q", responseQueue[0], "NotFound")
	}
}

func TestCollectRefsFromNode_ParameterRef(t *testing.T) {
	node := mustParseYAML(t, `
parameters:
  - $ref: "#/components/parameters/PageToken"
responses:
  "200":
    content:
      application/json:
        schema:
          $ref: "#/components/schemas/Result"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	if len(schemaQueue) != 1 {
		t.Fatalf("schemaQueue length = %d, want 1", len(schemaQueue))
	}
	if schemaQueue[0] != "Result" {
		t.Errorf("schemaQueue[0] = %q, want %q", schemaQueue[0], "Result")
	}
	if len(paramQueue) != 1 {
		t.Fatalf("paramQueue length = %d, want 1", len(paramQueue))
	}
	if paramQueue[0] != "PageToken" {
		t.Errorf("paramQueue[0] = %q, want %q", paramQueue[0], "PageToken")
	}
}

// --- pruneResponses ---

func TestPruneResponses(t *testing.T) {
	responses := mustParseYAML(t, `
"400":
  description: Bad Request
  content:
    application/json:
      schema:
        $ref: "#/components/schemas/Error"
"401":
  description: Unauthorized
"404":
  description: Not Found
TicketResponse:
  description: Ticket created
  content:
    application/json:
      schema:
        $ref: "#/components/schemas/Ticket"
`)

	reachable := map[string]bool{
		"400": true,
		"401": true,
	}

	removed := pruneResponses(responses, reachable)
	if removed != 2 { // 404 and TicketResponse
		t.Errorf("removed = %d, want 2", removed)
	}

	if getMappingValue(responses, "400") == nil {
		t.Error("400 should remain")
	}
	if getMappingValue(responses, "401") == nil {
		t.Error("401 should remain")
	}
	if getMappingValue(responses, "404") != nil {
		t.Error("404 should be pruned")
	}
	if getMappingValue(responses, "TicketResponse") != nil {
		t.Error("TicketResponse should be pruned")
	}
}

func TestPruneResponses_AllReachable(t *testing.T) {
	responses := mustParseYAML(t, `
"400":
  description: Bad Request
"500":
  description: Internal Server Error
`)

	reachable := map[string]bool{"400": true, "500": true}
	removed := pruneResponses(responses, reachable)
	if removed != 0 {
		t.Errorf("removed = %d, want 0", removed)
	}
}

// --- collectReachableResponses ---

func TestCollectReachableResponses(t *testing.T) {
	paths := mustParseYAML(t, `
/users:
  get:
    responses:
      "200":
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/User"
      "400":
        $ref: "#/components/responses/400"
      "401":
        $ref: "#/components/responses/401"
  post:
    responses:
      "201":
        $ref: "#/components/responses/Created"
      "400":
        $ref: "#/components/responses/400"
`)

	reachable := collectReachableResponses(nil, paths)

	if !reachable["400"] {
		t.Error("400 should be reachable")
	}
	if !reachable["401"] {
		t.Error("401 should be reachable")
	}
	if !reachable["Created"] {
		t.Error("Created should be reachable")
	}
	// 200 is an inline response, not a $ref
	if reachable["200"] {
		t.Error("200 should not be in reachable (it's inline, not a ref)")
	}
}

func TestCollectReachableResponses_NilPaths(t *testing.T) {
	reachable := collectReachableResponses(nil, nil)
	if len(reachable) != 0 {
		t.Errorf("expected empty reachable set, got %d", len(reachable))
	}
}

// --- collectReachableSchemas with parameter refs ---

func TestCollectReachableSchemas_ParameterRef(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /items:
    get:
      parameters:
        - $ref: "#/components/parameters/PageToken"
      responses:
        "200":
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ItemList"
components:
  schemas:
    ItemList:
      type: object
    PageInfo:
      type: object
      properties:
        token:
          type: string
    Unused:
      type: object
  parameters:
    PageToken:
      name: pageToken
      in: query
      schema:
        $ref: "#/components/schemas/PageInfo"
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatal(err)
	}
	doc := root.Content[0]

	pathsNode := findMapValue(doc, "paths")
	schemasNode := findMapValue(doc, "components", "schemas")

	reachable := collectReachableSchemas(doc, pathsNode, schemasNode)

	if !reachable["ItemList"] {
		t.Error("ItemList should be reachable directly")
	}
	if !reachable["PageInfo"] {
		t.Error("PageInfo should be reachable via components/parameters/PageToken")
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

func TestCollectRefsFromNode_MixedRefs(t *testing.T) {
	node := mustParseYAML(t, `
"200":
  content:
    application/json:
      schema:
        $ref: "#/components/schemas/User"
"400":
  $ref: "#/components/responses/BadRequest"
"500":
  $ref: "#/components/responses/InternalError"
`)

	var schemaQueue, responseQueue, paramQueue []string
	collectRefsFromNode(node, &schemaQueue, &responseQueue, &paramQueue)

	if len(schemaQueue) != 1 || schemaQueue[0] != "User" {
		t.Errorf("schemaQueue = %v, want [User]", schemaQueue)
	}
	if len(responseQueue) != 2 {
		t.Errorf("responseQueue length = %d, want 2", len(responseQueue))
	}
}

func TestCollectReachableSchemas_HeaderRef(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /things:
    post:
      responses:
        "201":
          description: Created
          headers:
            Request-Id:
              $ref: "#/components/headers/RequestID"
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Thing"
components:
  schemas:
    Thing:
      type: object
    RequestID:
      type: string
    Unused:
      type: object
  headers:
    RequestID:
      description: Request id
      schema:
        $ref: "#/components/schemas/RequestID"
`
	reachable := reachableFromSpec(t, spec)

	if !reachable["Thing"] {
		t.Error("Thing should be reachable directly")
	}
	if !reachable["RequestID"] {
		t.Error("RequestID should be reachable via components/headers/RequestID")
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

func TestCollectReachableSchemas_RequestBodyRef(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /things:
    post:
      requestBody:
        $ref: "#/components/requestBodies/ThingBody"
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Thing"
components:
  schemas:
    Thing:
      type: object
    ThingInput:
      type: object
    Unused:
      type: object
  requestBodies:
    ThingBody:
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/ThingInput"
`
	reachable := reachableFromSpec(t, spec)

	if !reachable["ThingInput"] {
		t.Error("ThingInput should be reachable via components/requestBodies/ThingBody")
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

func TestCollectReachableSchemas_OtherComponentChain(t *testing.T) {
	// response -> header -> header (cycle) -> schema, plus a schema that
	// reaches a requestBody which refs another schema.
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /things:
    get:
      responses:
        "200":
          $ref: "#/components/responses/Ok"
components:
  schemas:
    Deep:
      type: object
    Other:
      type: object
    Unused:
      type: object
  responses:
    Ok:
      description: OK
      headers:
        A:
          $ref: "#/components/headers/A"
  headers:
    A:
      description: a
      schema:
        $ref: "#/components/schemas/Deep"
      examples:
        loop:
          $ref: "#/components/headers/B"
    B:
      description: b
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Other"
      examples:
        loop:
          $ref: "#/components/headers/A"
`
	reachable := reachableFromSpec(t, spec)

	for _, name := range []string{"Deep", "Other"} {
		if !reachable[name] {
			t.Errorf("%s should be reachable through the headers chain", name)
		}
	}
	if reachable["Unused"] {
		t.Error("Unused should not be reachable")
	}
}

func TestCollectOtherRefs(t *testing.T) {
	node := mustParseYAML(t, `
a:
  $ref: "#/components/headers/H"
b:
  - $ref: "#/components/requestBodies/R"
  - $ref: "#/components/examples/E"
c:
  $ref: "#/components/schemas/S"
d:
  $ref: "#/components/responses/Resp"
e:
  $ref: "#/components/parameters/P"
f:
  $ref: "other.yaml#/components/headers/X"
g:
  $ref: "#/components/headers"
`)

	var got []componentRef
	collectOtherRefs(node, &got)

	want := []componentRef{
		{"headers", "H"},
		{"requestBodies", "R"},
		{"examples", "E"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ref %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func reachableFromSpec(t *testing.T, spec string) map[string]bool {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatal(err)
	}
	doc := root.Content[0]
	return collectReachableSchemas(doc, findMapValue(doc, "paths"), findMapValue(doc, "components", "schemas"))
}

func TestPruneSequence_ResponseReferencedThroughCallback(t *testing.T) {
	spec := `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /things:
    post:
      callbacks:
        onEvent:
          $ref: "#/components/callbacks/Event"
      responses:
        "201":
          description: Created
components:
  schemas:
    AckBody:
      type: object
    Unused:
      type: object
  responses:
    Ack:
      description: Ack
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/AckBody"
    Unused:
      description: Unused
  callbacks:
    Event:
      "{$request.body#/url}":
        post:
          responses:
            "200":
              $ref: "#/components/responses/Ack"
`
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatal(err)
	}
	doc := root.Content[0]
	pathsNode := findMapValue(doc, "paths")
	schemasNode := findMapValue(doc, "components", "schemas")
	responsesNode := findMapValue(doc, "components", "responses")

	pruneSchemas(schemasNode, collectReachableSchemas(doc, pathsNode, schemasNode))
	pruneResponses(responsesNode, collectReachableResponses(doc, pathsNode))

	if getMappingValue(schemasNode, "AckBody") == nil {
		t.Error("AckBody schema should be kept")
	}
	if getMappingValue(responsesNode, "Ack") == nil {
		t.Error("Ack response is referenced through a callback and must not be pruned (dangling $ref)")
	}
	if getMappingValue(responsesNode, "Unused") != nil {
		t.Error("Unused response should be pruned")
	}
}

// --- pruneComponentKind / collectReachableComponents ---

const unmappedParameterSpec = `
openapi: "3.0.1"
info:
  title: Test
  version: v1
paths:
  /a/{id}:
    get:
      parameters:
        - $ref: "#/components/parameters/Id"
      requestBody:
        $ref: "#/components/requestBodies/Body"
      responses:
        "200":
          description: OK
          headers:
            X-Rate:
              $ref: "#/components/headers/Rate"
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/A"
  /b:
    get:
      parameters:
        - $ref: "#/components/parameters/Region"
      requestBody:
        $ref: "#/components/requestBodies/OtherBody"
      responses:
        "200":
          description: OK
          headers:
            X-Other:
              $ref: "#/components/headers/Other"
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/B"
components:
  securitySchemes:
    apiKey:
      type: apiKey
      name: key
      in: header
  parameters:
    Id:
      name: id
      in: path
      required: true
      schema:
        type: string
    Region:
      name: region
      in: query
      schema:
        $ref: "#/components/schemas/Region"
  requestBodies:
    Body:
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/A"
    OtherBody:
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/OtherPayload"
  headers:
    Rate:
      schema:
        type: integer
    Other:
      schema:
        $ref: "#/components/schemas/OtherHeader"
  schemas:
    A:
      type: object
    B:
      type: object
    Region:
      type: string
      enum: [us, eu]
    OtherPayload:
      type: object
    OtherHeader:
      type: string
`

func TestPruneComponentKinds_UnmappedOperation(t *testing.T) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(unmappedParameterSpec), &root); err != nil {
		t.Fatal(err)
	}
	doc := root.Content[0]

	// Retain only /a/{id}, as an unmapped /b would be pruned by prunePaths.
	pathsNode := findMapValue(doc, "paths")
	prunePaths(pathsNode, map[pathMethod]bool{{path: "/a/{id}", method: "get"}: true})
	schemasNode := findMapValue(doc, "components", "schemas")
	pruneSchemas(schemasNode, collectReachableSchemas(doc, pathsNode, schemasNode))

	reachable := collectReachableComponents(doc, pathsNode)
	removed := map[string]int{}
	for _, kind := range prunableComponentKinds {
		if kindNode := findMapValue(doc, "components", kind); kindNode != nil {
			removed[kind] = pruneComponentKind(kindNode, reachable[kind])
		}
	}

	for kind, want := range map[string]int{"parameters": 1, "requestBodies": 1, "headers": 1} {
		if removed[kind] != want {
			t.Errorf("removed %s = %d, want %d", kind, removed[kind], want)
		}
	}
	for _, name := range []string{"Id", "Region"} {
		got := getMappingValue(findMapValue(doc, "components", "parameters"), name) != nil
		if got != (name == "Id") {
			t.Errorf("parameter %s present = %v", name, got)
		}
	}
	if findMapValue(doc, "components", "securitySchemes", "apiKey") == nil {
		t.Error("securitySchemes entry was pruned")
	}

	// No $ref left in the document may dangle.
	var check func(n *yaml.Node)
	check = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i < len(n.Content)-1; i += 2 {
				if n.Content[i].Value == "$ref" {
					rest := strings.TrimPrefix(n.Content[i+1].Value, "#/")
					if findMapValue(doc, strings.Split(rest, "/")...) == nil {
						t.Errorf("dangling $ref %q", n.Content[i+1].Value)
					}
					continue
				}
				check(n.Content[i+1])
			}
			return
		}
		for _, c := range n.Content {
			check(c)
		}
	}
	check(doc)
}

// --- nested component references ---

// pruneSpec runs the same pruning sequence as main's Phase 5 on spec.
func pruneSpec(t *testing.T, spec string) *yaml.Node {
	t.Helper()
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(spec), &root); err != nil {
		t.Fatal(err)
	}
	doc := root.Content[0]
	inlineAliases(findMapValue(doc, "paths"))
	inlineAliases(findMapValue(doc, "components"))
	pathsNode := findMapValue(doc, "paths")
	schemasNode := findMapValue(doc, "components", "schemas")
	pruneSchemas(schemasNode, collectReachableSchemas(doc, pathsNode, schemasNode))
	reachable := collectReachableComponents(doc, pathsNode)
	for _, kind := range prunableComponentKinds {
		if kindNode := findMapValue(doc, "components", kind); kindNode != nil {
			pruneComponentKind(kindNode, reachable[kind])
		}
	}
	return doc
}

func TestPrune_NestedRefIntoParameter(t *testing.T) {
	doc := pruneSpec(t, `
openapi: "3.0.1"
info: {title: t, version: v1}
paths:
  /a:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/parameters/Id/schema"
components:
  parameters:
    Id:
      name: id
      in: query
      schema:
        type: string
  schemas: {}
`)
	if findMapValue(doc, "components", "parameters", "Id") == nil {
		t.Error("parameter Id was pruned although #/components/parameters/Id/schema is referenced")
	}
}

func TestPrune_NestedRefIntoHeader(t *testing.T) {
	doc := pruneSpec(t, `
openapi: "3.0.1"
info: {title: t, version: v1}
paths:
  /a:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/headers/Rate/schema"
components:
  headers:
    Rate:
      schema:
        type: integer
  schemas: {}
`)
	if findMapValue(doc, "components", "headers", "Rate") == nil {
		t.Error("header Rate was pruned although #/components/headers/Rate/schema is referenced")
	}
}

func TestPrune_NestedRefIntoSchema(t *testing.T) {
	doc := pruneSpec(t, `
openapi: "3.0.1"
info: {title: t, version: v1}
paths:
  /a:
    get:
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Outer/properties/inner"
components:
  schemas:
    Outer:
      type: object
      properties:
        inner:
          type: string
`)
	if findMapValue(doc, "components", "schemas", "Outer") == nil {
		t.Error("schema Outer was pruned although #/components/schemas/Outer/properties/inner is referenced")
	}
}

// --- YAML anchors ---

// TestPrune_AliasToPrunedAnchor checks the emitted YAML still loads, with the
// aliased value intact, when a retained node aliases an anchor defined inside a
// pruned component.
func TestPrune_AliasToPrunedAnchor(t *testing.T) {
	root := pruneSpec(t, `
openapi: "3.0.1"
info: {title: t, version: v1}
paths:
  /a:
    get:
      parameters:
        - $ref: "#/components/parameters/Kept"
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Kept"
components:
  parameters:
    Unused:
      name: unused
      in: query
      description: &desc shared text
      schema: {type: string}
    Kept:
      name: kept
      in: query
      description: *desc
      schema: {type: string}
  schemas:
    Unused:
      type: object
      description: &sdesc shared schema text
    Kept:
      type: object
      description: *sdesc
`)

	out, err := yaml.Marshal(root)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var reloaded yaml.Node
	if err := yaml.Unmarshal(out, &reloaded); err != nil {
		t.Fatalf("emitted YAML does not reload: %v\n%s", err, out)
	}
	doc := reloaded.Content[0]
	for path, want := range map[string][]string{
		"shared text":        {"components", "parameters", "Kept", "description"},
		"shared schema text": {"components", "schemas", "Kept", "description"},
	} {
		if got := findMapValue(doc, want...); got == nil || got.Value != path {
			t.Errorf("%v = %v, want %q", want, got, path)
		}
	}
}
