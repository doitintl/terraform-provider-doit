package main

import (
	"slices"
	"strings"
	"testing"
)

func TestVerifyMerge_CleanModifyAddDelete(t *testing.T) {
	t.Parallel()

	local := `openapi: 3.0.1
paths:
  /analytics/v1/local-only:
    get:
      summary: Local only path
  /analytics/v1/budgets:
    get:
      summary: List budgets
  /analytics/v1/legacy:
    get:
      summary: To be deleted
components:
  schemas:
    LocalOnly:
      type: object
    Budget:
      type: object
      properties:
        amount:
          type: number
          description: Old amount
    OldSchema:
      type: object
`

	base := `openapi: 3.0.1
paths:
  /analytics/v1/budgets:
    get:
      summary: List budgets
  /analytics/v1/legacy:
    get:
      summary: To be deleted
  /analytics/v1/upstream-only:
    get:
      summary: Upstream only path
components:
  schemas:
    Budget:
      type: object
      properties:
        amount:
          type: number
          description: Old amount
    OldSchema:
      type: object
    UpstreamOnly:
      type: object
`

	head := `openapi: 3.0.1
paths:
  /analytics/v1/budgets:
    get:
      summary: List all budgets
  /analytics/v1/budgets/new:
    post:
      summary: New path
  /analytics/v1/upstream-only:
    get:
      summary: Upstream only path
components:
  schemas:
    Budget:
      type: object
      properties:
        amount:
          type: number
          description: New amount
    BudgetRollover:
      type: object
      properties:
        enabled:
          type: boolean
    UpstreamOnly:
      type: object
`

	merged := `openapi: 3.0.1
paths:
  /analytics/v1/local-only:
    get:
      summary: Local only path
  /analytics/v1/budgets:
    get:
      summary: List all budgets
  /analytics/v1/budgets/new:
    post:
      summary: New path
components:
  schemas:
    LocalOnly:
      type: object
    Budget:
      type: object
      properties:
        amount:
          type: number
          description: New amount
    BudgetRollover:
      type: object
      properties:
        enabled:
          type: boolean
`

	res, err := VerifyMerge([]byte(local), []byte(base), []byte(head), []byte(merged))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Failures) != 0 {
		t.Fatalf("expected no failures, got: %v", res.Failures)
	}
	if len(res.Diverged) != 0 {
		t.Fatalf("expected no diverged entries, got: %v", res.Diverged)
	}

	wantTouched := []string{
		"components.schemas.Budget",
		"components.schemas.BudgetRollover",
		"components.schemas.OldSchema",
		`paths["/analytics/v1/budgets"]`,
		`paths["/analytics/v1/budgets/new"]`,
		`paths["/analytics/v1/legacy"]`,
	}
	if !slices.Equal(res.Touched, wantTouched) {
		t.Fatalf("touched mismatch:\ngot:  %v\nwant: %v", res.Touched, wantTouched)
	}
}

func TestVerifyMerge_SwappedBudgetSchemasDetected(t *testing.T) {
	t.Parallel()

	// Reproduces #483/#484: hunks for BudgetCreateRequest and BudgetUpdateRequest
	// were swapped by git apply, so each schema received the other's descriptions.
	base := `openapi: 3.0.1
components:
  schemas:
    BudgetCreateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
    BudgetUpdateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Budget period amount
        growthPerPeriod:
          type: number
          description: Periodical growth percentage in recurring budget
`

	head := `openapi: 3.0.1
components:
  schemas:
    BudgetCreateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Create amount description
        allowGrowth:
          type: boolean
          description: Create allowGrowth description
        growthPerPeriod:
          type: number
          description: Create growthPerPeriod description
    BudgetUpdateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Update amount description
        allowGrowth:
          type: boolean
          description: Update allowGrowth description
        growthPerPeriod:
          type: number
          description: Update growthPerPeriod description
`

	// Swapped merge result (Create got Update descriptions and vice versa).
	swapped := `openapi: 3.0.1
components:
  schemas:
    BudgetCreateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Update amount description
        allowGrowth:
          type: boolean
          description: Update allowGrowth description
        growthPerPeriod:
          type: number
          description: Update growthPerPeriod description
    BudgetUpdateRequest:
      type: object
      properties:
        amount:
          type: number
          description: Create amount description
        allowGrowth:
          type: boolean
          description: Create allowGrowth description
        growthPerPeriod:
          type: number
          description: Create growthPerPeriod description
`

	res, err := VerifyMerge([]byte(base), []byte(base), []byte(head), []byte(swapped))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Failures) != 2 {
		t.Fatalf("expected 2 failures, got %d: %v", len(res.Failures), res.Failures)
	}
	gotPaths := []string{res.Failures[0].Path, res.Failures[1].Path}
	wantPaths := []string{
		"components.schemas.BudgetCreateRequest",
		"components.schemas.BudgetUpdateRequest",
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("failure paths mismatch:\ngot:  %v\nwant: %v", gotPaths, wantPaths)
	}
}

func TestVerifyMerge_StrayModificationToUntouchedSchemaAndPath(t *testing.T) {
	t.Parallel()

	base := `openapi: 3.0.1
paths:
  /analytics/v1/untouched:
    get:
      summary: Original summary
components:
  schemas:
    Touched:
      type: object
      description: Old
    Untouched:
      type: object
      description: Keep me
`

	head := `openapi: 3.0.1
paths:
  /analytics/v1/untouched:
    get:
      summary: Original summary
components:
  schemas:
    Touched:
      type: object
      description: New
    Untouched:
      type: object
      description: Keep me
`

	corrupted := `openapi: 3.0.1
paths:
  /analytics/v1/untouched:
    get:
      summary: Stray edit
components:
  schemas:
    Touched:
      type: object
      description: New
    Untouched:
      type: object
      description: Corrupted by stray hunk
`

	res, err := VerifyMerge([]byte(base), []byte(base), []byte(head), []byte(corrupted))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Failures) != 2 {
		t.Fatalf("expected 2 failures, got %d: %v", len(res.Failures), res.Failures)
	}
	gotPaths := []string{res.Failures[0].Path, res.Failures[1].Path}
	wantPaths := []string{
		"components.schemas.Untouched",
		`paths["/analytics/v1/untouched"]`,
	}
	if !slices.Equal(gotPaths, wantPaths) {
		t.Fatalf("failure paths mismatch:\ngot:  %v\nwant: %v", gotPaths, wantPaths)
	}
}

func TestVerifyMerge_DivergedSchemaVerifiedFieldByField(t *testing.T) {
	t.Parallel()

	local := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        localField:
          type: string
          description: Local independent edit
        amount:
          type: number
          description: Base amount
`

	base := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        localField:
          type: string
          description: Base localField
        amount:
          type: number
          description: Base amount
`

	head := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        localField:
          type: string
          description: Base localField
        amount:
          type: number
          description: PR updated amount
`

	t.Run("both changes preserved passes with diverged note", func(t *testing.T) {
		t.Parallel()
		merged := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        localField:
          type: string
          description: Local independent edit
        amount:
          type: number
          description: PR updated amount
`
		res, err := VerifyMerge([]byte(local), []byte(base), []byte(head), []byte(merged))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Failures) != 0 {
			t.Fatalf("expected no failures, got: %v", res.Failures)
		}
		wantDiverged := []string{"components.schemas.Budget"}
		if !slices.Equal(res.Diverged, wantDiverged) {
			t.Fatalf("diverged mismatch: got %v, want %v", res.Diverged, wantDiverged)
		}
	})

	t.Run("PR field wrong inside diverged schema fails at property path", func(t *testing.T) {
		t.Parallel()
		mergedWrong := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        localField:
          type: string
          description: Local independent edit
        amount:
          type: number
          description: Wrong amount
`
		res, err := VerifyMerge([]byte(local), []byte(base), []byte(head), []byte(mergedWrong))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Failures) != 1 || res.Failures[0].Path != "components.schemas.Budget.properties.amount" {
			t.Fatalf("expected failure at Budget.properties.amount, got: %v", res.Failures)
		}
	})

	t.Run("local field clobbered inside diverged schema fails at property path", func(t *testing.T) {
		t.Parallel()
		mergedClobbered := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        localField:
          type: string
          description: Base localField
        amount:
          type: number
          description: PR updated amount
`
		res, err := VerifyMerge([]byte(local), []byte(base), []byte(head), []byte(mergedClobbered))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Failures) != 1 || res.Failures[0].Path != "components.schemas.Budget.properties.localField" {
			t.Fatalf("expected failure at Budget.properties.localField, got: %v", res.Failures)
		}
	})
}

func TestVerifyMerge_DivergedSequenceAndScalar(t *testing.T) {
	t.Parallel()

	local := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      required:
        - localReq
        - name
        - scope
`
	base := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      required:
        - name
        - scope
`
	head := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      required:
        - name
        - scope
        - prReq
`

	t.Run("clean sequence merge passes", func(t *testing.T) {
		t.Parallel()
		merged := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      required:
        - localReq
        - name
        - scope
        - prReq
`
		res, err := VerifyMerge([]byte(local), []byte(base), []byte(head), []byte(merged))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Failures) != 0 {
			t.Fatalf("expected no failures, got: %v", res.Failures)
		}
	})

	t.Run("PR sequence change silently dropped fails", func(t *testing.T) {
		t.Parallel()
		res, err := VerifyMerge([]byte(local), []byte(base), []byte(head), []byte(local))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Failures) != 1 || res.Failures[0].Path != "components.schemas.Budget.required" {
			t.Fatalf("expected failure on Budget.required, got: %v", res.Failures)
		}
	})
}

func TestVerifyMerge_IdempotentAlreadyApplied(t *testing.T) {
	t.Parallel()

	base := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      description: Old
`
	head := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      description: New
`
	res, err := VerifyMerge([]byte(head), []byte(base), []byte(head), []byte(head))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Failures) != 0 {
		t.Fatalf("expected no failures, got: %v", res.Failures)
	}
}

func TestVerifyMerge_YAMLAnchorsAndAliases(t *testing.T) {
	t.Parallel()

	base := `openapi: 3.0.1
components:
  x-doit-doc-anchors:
    sharedDesc: &sharedDesc Old description
  schemas:
    Budget:
      type: object
      description: *sharedDesc
`
	head := `openapi: 3.0.1
components:
  x-doit-doc-anchors:
    sharedDesc: &sharedDesc Updated description
  schemas:
    Budget:
      type: object
      description: *sharedDesc
`

	t.Run("anchor update verified cleanly", func(t *testing.T) {
		t.Parallel()
		res, err := VerifyMerge([]byte(base), []byte(base), []byte(head), []byte(head))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Failures) != 0 {
			t.Fatalf("expected no failures, got: %v", res.Failures)
		}
	})

	t.Run("broken alias in merged fails YAML parse", func(t *testing.T) {
		t.Parallel()
		broken := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      description: *missingAnchor
`
		_, err := VerifyMerge([]byte(base), []byte(base), []byte(head), []byte(broken))
		if err == nil || !strings.Contains(err.Error(), "merged spec") {
			t.Fatalf("expected merged spec YAML parse error, got: %v", err)
		}
	})
}

func TestVerifyMerge_DuplicateKeysIntroducedByMerge(t *testing.T) {
	t.Parallel()

	base := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        amount:
          type: number
`
	head := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        amount:
          type: number
          description: Updated
`
	dupMerged := `openapi: 3.0.1
components:
  schemas:
    Budget:
      type: object
      properties:
        amount:
          type: number
        amount:
          type: number
          description: Updated
`
	res, err := VerifyMerge([]byte(base), []byte(base), []byte(head), []byte(dupMerged))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Failures) == 0 {
		t.Fatal("expected duplicate key failure, got none")
	}
	if !strings.Contains(res.Failures[0].Reason, "duplicate YAML mapping key") {
		t.Fatalf("expected duplicate key reason, got: %v", res.Failures)
	}
}

func TestVerifyMerge_NullScalarEquivalence(t *testing.T) {
	t.Parallel()

	base := `openapi: 3.0.1
components:
  schemas:
    Foo:
      default: ~
`
	head := `openapi: 3.0.1
components:
  schemas:
    Foo:
      default: null
      description: Added
`
	merged := `openapi: 3.0.1
components:
  schemas:
    Foo:
      default:
      description: Added
`
	res, err := VerifyMerge([]byte(base), []byte(base), []byte(head), []byte(merged))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Failures) != 0 {
		t.Fatalf("expected no failures for equivalent null representations, got: %v", res.Failures)
	}
}
