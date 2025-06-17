package api

import (
	"bytes"
	"component-service/db"
	"component-service/models"
	"component-service/store"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	_ "github.com/lib/pq" // DB driver
)

// testRouter is the router to be tested.
var testRouter http.Handler

// Global store for tests
var testAPIStore *store.ComponentStore

func TestMain(m *testing.M) {
	// Setup: Initialize database for tests
	if os.Getenv("DB_HOST") == "" || os.Getenv("DB_USER") == "" || os.Getenv("DB_NAME") == "" {
		log.Println("Skipping API integration tests: DB_HOST, DB_USER, or DB_NAME environment variables not set.")
		os.Exit(0) // Exit if DB is not configured, as all API tests depend on it.
	}

	db.InitDB() // Initialize connection using env vars
	testAPIStore = &store.ComponentStore{} // Used by handlers, and directly for setup/assertions

	// Setup router
	mux := http.NewServeMux()
	mux.HandleFunc("/components/", ComponentsHandler) // Register the main handler
	testRouter = mux

	// Clean database before running tests
	clearComponentsTableForAPITests()

	exitCode := m.Run()

	os.Exit(exitCode)
}

func clearComponentsTableForAPITests() {
	if db.DB == nil {
		log.Fatal("Cannot clear table: DB connection not initialized for API tests.")
	}
	// Using TRUNCATE for efficiency and to reset sequences if any.
	// CASCADE is important if there are foreign keys from other tables not managed here.
	_, err := db.DB.Exec("TRUNCATE components RESTART IDENTITY CASCADE")
	if err != nil {
		log.Fatalf("Failed to clear components table for API tests: %v", err)
	}
}

// Helper to create a component directly via store for test setup
func createTestComponentDirectly(t *testing.T, name string, description string, parentID sql.NullInt64) *models.Component {
	comp := &models.Component{
		Name:        name,
		Description: description,
		ParentID:    parentID,
	}
	id, err := testAPIStore.CreateComponent(comp) // Use the global testAPIStore
	assert.NoError(t, err)
	comp.ID = id
	// Fetch to get all fields, especially timestamps
	createdComp, err := testAPIStore.GetComponentByID(id)
	assert.NoError(t, err)
	assert.NotNil(t, createdComp)
	return createdComp
}


func TestAPIComponentsFlow(t *testing.T) {
	if db.DB == nil {
		t.Skip("Skipping API test: DB connection not initialized.")
	}
	clearComponentsTableForAPITests() // Clear before each major test flow if needed or rely on TestMain

	var createdRootID int64
	var createdChildID int64

	// 1. Create a root component
	t.Run("POST_CreateRootComponent", func(t *testing.T) {
		payload := `{"name": "APIRoot", "description": "Root via API"}`
		req, _ := http.NewRequest(http.MethodPost, "/components/", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		var comp models.Component
		err := json.Unmarshal(rr.Body.Bytes(), &comp)
		assert.NoError(t, err)
		assert.Equal(t, "APIRoot", comp.Name)
		assert.NotZero(t, comp.ID)
		assert.False(t, comp.ParentID.Valid)
		createdRootID = comp.ID
	})

	// 2. Get the root component
	t.Run("GET_RootComponentByID", func(t *testing.T) {
		assumeIDSet(t, createdRootID, "createdRootID")
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/components/%d", createdRootID), nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var comp models.Component
		err := json.Unmarshal(rr.Body.Bytes(), &comp)
		assert.NoError(t, err)
		assert.Equal(t, "APIRoot", comp.Name)
		assert.Equal(t, createdRootID, comp.ID)
		assert.NotEmpty(t, comp.CreatedAt) // Check timestamps are populated
		assert.NotEmpty(t, comp.UpdatedAt)
	})

	// 3. Create a child component
	t.Run("POST_CreateChildComponent", func(t *testing.T) {
		assumeIDSet(t, createdRootID, "createdRootID for child creation")
		payload := fmt.Sprintf(`{"name": "APIChild", "description": "Child via API", "parent_id": %d}`, createdRootID)
		req, _ := http.NewRequest(http.MethodPost, "/components/", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)
		var comp models.Component
		err := json.Unmarshal(rr.Body.Bytes(), &comp)
		assert.NoError(t, err)
		assert.Equal(t, "APIChild", comp.Name)
		assert.NotZero(t, comp.ID)
		assert.True(t, comp.ParentID.Valid)
		assert.Equal(t, createdRootID, comp.ParentID.Int64)
		createdChildID = comp.ID
	})

	// 3.5 Create another root component for list testing
	t.Run("POST_CreateAnotherRootComponent", func(t *testing.T) {
		payload := `{"name": "APIRoot2", "description": "Second Root via API"}`
		req, _ := http.NewRequest(http.MethodPost, "/components/", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusCreated, rr.Code)
	})


	// 4. List all components
	t.Run("GET_ListAllComponents", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/components/", nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var comps []*models.Component
		err := json.Unmarshal(rr.Body.Bytes(), &comps)
		assert.NoError(t, err)
		assert.Len(t, comps, 3, "Should be 3 components: Root, Child, Root2")
	})

	// 5. List children of the root component
	t.Run("GET_ListChildComponents", func(t *testing.T) {
		assumeIDSet(t, createdRootID, "createdRootID for listing children")
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/components/%d/children", createdRootID), nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var children []*models.Component
		err := json.Unmarshal(rr.Body.Bytes(), &children)
		assert.NoError(t, err)
		assert.Len(t, children, 1)
		assert.Equal(t, createdChildID, children[0].ID)
		assert.Equal(t, "APIChild", children[0].Name)
	})

	// 6. Update the child component (e.g., change its name and make it a root)
	t.Run("PUT_UpdateChildComponent", func(t *testing.T) {
		assumeIDSet(t, createdChildID, "createdChildID for update")
		payload := `{"name": "UpdatedAPIChild", "description": "Updated Child Desc", "parent_id": null}` // Make it a root
		req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/components/%d", createdChildID), bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var comp models.Component
		err := json.Unmarshal(rr.Body.Bytes(), &comp)
		assert.NoError(t, err)
		assert.Equal(t, "UpdatedAPIChild", comp.Name)
		assert.False(t, comp.ParentID.Valid, "ParentID should now be null")
	})

	// 7. Delete the first root component
	t.Run("DELETE_RootComponent", func(t *testing.T) {
		assumeIDSet(t, createdRootID, "createdRootID for delete")
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/components/%d", createdRootID), nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var respMsg map[string]string
		err := json.Unmarshal(rr.Body.Bytes(), &respMsg)
		assert.NoError(t, err)
		assert.Equal(t, "Component deleted successfully", respMsg["message"])

		// Verify it's gone
		reqGet, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/components/%d", createdRootID), nil)
		rrGet := httptest.NewRecorder()
		testRouter.ServeHTTP(rrGet, reqGet)
		assert.Equal(t, http.StatusNotFound, rrGet.Code)
	})

	// 8. Check if child (now a root) still exists and its parent_id is null (already verified by update, but good check)
    // The child component (ID: createdChildID) was updated to have parent_id = null.
    // The original parent (ID: createdRootID) was deleted.
    // The schema has ON DELETE SET NULL for parent_id. If the child's parent_id had *not* been updated to null
    // *before* the parent was deleted, then ON DELETE SET NULL would have made it null.
    // Since we explicitly set it to null during update, this check confirms it's still accessible and a root.
	t.Run("GET_VerifyChildAfterParentDeleteAndUpdate", func(t *testing.T) {
		assumeIDSet(t, createdChildID, "createdChildID for verification")
		req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/components/%d", createdChildID), nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		var comp models.Component
		err := json.Unmarshal(rr.Body.Bytes(), &comp)
		assert.NoError(t, err)
		assert.Equal(t, "UpdatedAPIChild", comp.Name) // Name was updated
		assert.False(t, comp.ParentID.Valid)      // ParentID was set to null via PUT
	})


	// 9. Test Bad Requests
	t.Run("POST_CreateComponent_BadRequest_NoName", func(t *testing.T) {
		payload := `{"description": "Missing name"}`
		req, _ := http.NewRequest(http.MethodPost, "/components/", bytes.NewBufferString(payload))
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("GET_Component_InvalidIDFormat", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/components/notanumber", nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code) // Our handler logic catches bad ID format
	})

	t.Run("GET_Component_NotFound", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/components/999999", nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	// Test listing children of a non-existent parent
	t.Run("GET_ListChildren_NonExistentParent", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/components/999999/children", nil)
		rr := httptest.NewRecorder()
		testRouter.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code, "Should be 404 if parent component does not exist")
	})
}

// assumeIDSet checks if an ID is non-zero, failing the test if it's zero,
// as it indicates a setup step (like creation) might have failed.
func assumeIDSet(t *testing.T, id int64, idName string) {
	t.Helper() // Marks this as a helper function for test logging
	if id == 0 {
		t.Fatalf("%s was not set. Previous step likely failed.", idName)
	}
}

// Add more specific tests for edge cases or error conditions as needed
// e.g., invalid JSON payload, attempting to update non-existent component, etc.
