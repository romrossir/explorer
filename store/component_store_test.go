package store

import (
	"component-service/db"
	"component-service/models"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	_ "github.com/lib/pq" // Driver for sql.Open if not already imported by db package in test scope
)

var testStore *ComponentStore

func TestMain(m *testing.M) {
	// Setup: Initialize database for tests
	// IMPORTANT: These tests require a running PostgreSQL instance configured via environment variables.
	// It's highly recommended to use a DEDICATED TEST DATABASE to avoid data loss.
	// Set these environment variables before running tests:
	// DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME (for the test database), DB_SSLMODE

	// Check for essential DB env vars
	if os.Getenv("DB_HOST") == "" || os.Getenv("DB_USER") == "" || os.Getenv("DB_NAME") == "" {
		log.Println("Skipping database tests: DB_HOST, DB_USER, or DB_NAME environment variables not set.")
		log.Println("Please set DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE to run these tests.")
		// Do not exit here, allow other non-DB tests in the package if any.
		// For this file, all tests are DB tests, so they will be skipped by `setupTestDB`.
	} else {
		db.InitDB() // Initialize connection using env vars
		testStore = &ComponentStore{}

		// Optional: Clean up or prepare the database before tests
		// e.g., clear tables, or ensure schema is applied.
		// For simplicity, we assume schema.sql has been applied manually.
		// It's better to use a migration tool or auto-apply schema in InitDB for tests.
		clearComponentsTableForTest()
	}

	// Run tests
	exitCode := m.Run()

	// Teardown: Clean up database after tests (optional, depends on strategy)
	// clearComponentsTableForTest() // Uncomment if you want to clear after all tests in the package

	os.Exit(exitCode)
}

func clearComponentsTableForTest() {
	if db.DB == nil {
		log.Println("Skipping table clear: DB connection not initialized.")
		return
	}
	// Order of deletion matters if there are foreign key constraints.
	// If components can have children that are also components,
	// you might need to delete them in a specific order or use CASCADE.
	// For now, we assume parent_id ON DELETE SET NULL handles this,
	// or we delete in an order if necessary (e.g., multiple passes or by depth).
	// A simpler approach for full cleanup is TRUNCATE...CASCADE if supported and appropriate.
	_, err := db.DB.Exec("DELETE FROM components") // This will be slow on large tables
	// _, err := db.DB.Exec("TRUNCATE components RESTART IDENTITY CASCADE") // More efficient for full clear
	if err != nil {
		log.Fatalf("Failed to clear components table: %v", err)
	}
	log.Println("Components table cleared for tests.")
}

// Helper to create a component for testing, ensuring cleanup
func createTestComponent(t *testing.T, name string, description string, parentID sql.NullInt64) *models.Component {
	if db.DB == nil {
		t.Skip("Skipping test: DB connection not initialized.")
	}
	comp := &models.Component{
		Name:        name,
		Description: description,
		ParentID:    parentID,
	}
	id, err := testStore.CreateComponent(comp)
	assert.NoError(t, err)
	assert.NotZero(t, id)
	comp.ID = id

	// Fetch to get DB-generated timestamps
	createdComp, err := testStore.GetComponentByID(id)
	assert.NoError(t, err)
	assert.NotNil(t, createdComp)
	return createdComp
}


func TestCreateComponent(t *testing.T) {
	if db.DB == nil {
		t.Skip("Skipping test: DB connection not initialized.")
	}
	clearComponentsTableForTest()

	t.Run("Create root component", func(t *testing.T) {
		comp := &models.Component{
			Name:        "Root Component",
			Description: "This is a root component.",
			ParentID:    sql.NullInt64{Valid: false}, // No parent
		}
		id, err := testStore.CreateComponent(comp)
		assert.NoError(t, err)
		assert.NotZero(t, id)

		createdComp, err := testStore.GetComponentByID(id)
		assert.NoError(t, err)
		assert.NotNil(t, createdComp)
		assert.Equal(t, "Root Component", createdComp.Name)
		assert.False(t, createdComp.ParentID.Valid, "Root component should have null ParentID")
	})

	t.Run("Create child component", func(t *testing.T) {
		clearComponentsTableForTest() // Clear for this specific sub-test
		parentComp := createTestComponent(t, "Parent For Child", "Parent desc", sql.NullInt64{Valid: false})

		childComp := &models.Component{
			Name:        "Child Component",
			Description: "This is a child component.",
			ParentID:    sql.NullInt64{Int64: parentComp.ID, Valid: true},
		}
		id, err := testStore.CreateComponent(childComp)
		assert.NoError(t, err)
		assert.NotZero(t, id)

		createdChild, err := testStore.GetComponentByID(id)
		assert.NoError(t, err)
		assert.NotNil(t, createdChild)
		assert.Equal(t, "Child Component", createdChild.Name)
		assert.True(t, createdChild.ParentID.Valid)
		assert.Equal(t, parentComp.ID, createdChild.ParentID.Int64)
	})
}

func TestGetComponentByID(t *testing.T) {
	if db.DB == nil {
		t.Skip("Skipping test: DB connection not initialized.")
	}
	clearComponentsTableForTest()

	comp := createTestComponent(t, "TestGet", "DescGet", sql.NullInt64{Valid: false})

	t.Run("Get existing component", func(t *testing.T) {
		foundComp, err := testStore.GetComponentByID(comp.ID)
		assert.NoError(t, err)
		assert.NotNil(t, foundComp)
		assert.Equal(t, comp.ID, foundComp.ID)
		assert.Equal(t, comp.Name, foundComp.Name)
		// Timestamps might be slightly different due to formatting/precision,
		// so check for non-empty and parseable if strict equality is needed.
		assert.NotEmpty(t, foundComp.CreatedAt)
		assert.NotEmpty(t, foundComp.UpdatedAt)
	})

	t.Run("Get non-existent component", func(t *testing.T) {
		_, err := testStore.GetComponentByID(99999) // Non-existent ID
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestUpdateComponent(t *testing.T) {
	if db.DB == nil {
		t.Skip("Skipping test: DB connection not initialized.")
	}
	clearComponentsTableForTest()

	comp := createTestComponent(t, "TestUpdate", "DescUpdate", sql.NullInt64{Valid: false})
	parentForUpdate := createTestComponent(t, "ParentForUpdate", "ParentForUpdateDesc", sql.NullInt64{Valid: false})


	t.Run("Update existing component", func(t *testing.T) {
		comp.Name = "Updated Name"
		comp.Description = "Updated Description"
		comp.ParentID = sql.NullInt64{Int64: parentForUpdate.ID, Valid: true}

		// Need to parse string time back to time.Time for UpdateComponent if models.Component expects time.Time
		// However, our store's UpdateComponent takes *models.Component, and models.Component has string times.
		// The store internally uses time.Now() for updated_at.
		// The component passed to UpdateComponent primarily provides Name, Description, ParentID.

		err := testStore.UpdateComponent(comp.ID, comp)
		assert.NoError(t, err)

		updatedComp, err := testStore.GetComponentByID(comp.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Name", updatedComp.Name)
		assert.Equal(t, "Updated Description", updatedComp.Description)
		assert.True(t, updatedComp.ParentID.Valid)
		assert.Equal(t, parentForUpdate.ID, updatedComp.ParentID.Int64)

		// Check if updated_at changed (tricky to check exact time)
		// For simplicity, we assume the trigger works or store sets it.
		// A more robust check would involve comparing UpdatedAt before and after.
		originalUpdatedAt, _ := time.Parse(time.RFC3339, comp.UpdatedAt)
		currentUpdatedAt, _ := time.Parse(time.RFC3339, updatedComp.UpdatedAt)
		assert.True(t, currentUpdatedAt.After(originalUpdatedAt) || currentUpdatedAt.Equal(originalUpdatedAt), "UpdatedAt should be same or later")

	})

	t.Run("Update non-existent component", func(t *testing.T) {
		nonExistentComp := &models.Component{Name: "NonExistent"}
		err := testStore.UpdateComponent(88888, nonExistentComp) // Non-existent ID
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found for update")
	})
}

func TestDeleteComponent(t *testing.T) {
	if db.DB == nil {
		t.Skip("Skipping test: DB connection not initialized.")
	}
	clearComponentsTableForTest()

	compToDelete := createTestComponent(t, "TestDelete", "DescDelete", sql.NullInt64{Valid: false})

	t.Run("Delete existing component", func(t *testing.T) {
		err := testStore.DeleteComponent(compToDelete.ID)
		assert.NoError(t, err)

		_, err = testStore.GetComponentByID(compToDelete.ID)
		assert.Error(t, err, "Expected error when getting deleted component")
		assert.Contains(t, err.Error(), "not found", "Error message should indicate not found")
	})

	t.Run("Delete non-existent component", func(t *testing.T) {
		err := testStore.DeleteComponent(77777) // Non-existent ID
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found for deletion")
	})

	t.Run("Delete parent component - check child's parent_id becomes NULL", func(t *testing.T) {
		clearComponentsTableForTest()
		parent := createTestComponent(t, "Parent For Deletion Test", "Parent Desc", sql.NullInt64{Valid: false})
		child := createTestComponent(t, "Child of Deleted Parent", "Child Desc", sql.NullInt64{Int64: parent.ID, Valid: true})

		// Ensure child's parent is correctly set
		assert.True(t, child.ParentID.Valid)
		assert.Equal(t, parent.ID, child.ParentID.Int64)

		// Delete the parent
		err := testStore.DeleteComponent(parent.ID)
		assert.NoError(t, err)

		// Fetch the child again
		updatedChild, err := testStore.GetComponentByID(child.ID)
		assert.NoError(t, err)
		assert.NotNil(t, updatedChild)
		assert.False(t, updatedChild.ParentID.Valid, "Child's ParentID should be NULL after parent deletion due to ON DELETE SET NULL")
	})
}

func TestListComponents(t *testing.T) {
	if db.DB == nil {
		t.Skip("Skipping test: DB connection not initialized.")
	}
	clearComponentsTableForTest()

	createTestComponent(t, "ListComp1", "Desc1", sql.NullInt64{Valid: false})
	createTestComponent(t, "ListComp2", "Desc2", sql.NullInt64{Valid: false})

	components, err := testStore.ListComponents()
	assert.NoError(t, err)
	assert.Len(t, components, 2)
}

func TestListChildComponents(t *testing.T) {
	if db.DB == nil {
		t.Skip("Skipping test: DB connection not initialized.")
	}
	clearComponentsTableForTest()

	parent1 := createTestComponent(t, "Parent1", "P1Desc", sql.NullInt64{Valid: false})
	_ = createTestComponent(t, "Child1P1", "C1P1Desc", sql.NullInt64{Int64: parent1.ID, Valid: true})
	_ = createTestComponent(t, "Child2P1", "C2P1Desc", sql.NullInt64{Int64: parent1.ID, Valid: true})

	parent2 := createTestComponent(t, "Parent2", "P2Desc", sql.NullInt64{Valid: false})
	_ = createTestComponent(t, "Child1P2", "C1P2Desc", sql.NullInt64{Int64: parent2.ID, Valid: true})

	// Create a grandchild to ensure only direct children are listed
	// child1P1 := // Assuming we got Child1P1 from createTestComponent
	// _ = createTestComponent(t, "GrandChild1P1", "GC1P1Desc", sql.NullInt64{Int64: child1P1.ID, Valid: true})


	t.Run("List children for parent1", func(t *testing.T) {
		children, err := testStore.ListChildComponents(parent1.ID)
		assert.NoError(t, err)
		assert.Len(t, children, 2)
		for _, child := range children {
			assert.True(t, child.ParentID.Valid)
			assert.Equal(t, parent1.ID, child.ParentID.Int64)
		}
	})

	t.Run("List children for parent2", func(t *testing.T) {
		children, err := testStore.ListChildComponents(parent2.ID)
		assert.NoError(t, err)
		assert.Len(t, children, 1)
		assert.Equal(t, parent2.ID, children[0].ParentID.Int64)
	})

	t.Run("List children for a component with no children", func(t *testing.T) {
		noChildrenParent := createTestComponent(t, "NoChildren", "NoChildrenDesc", sql.NullInt64{Valid: false})
		children, err := testStore.ListChildComponents(noChildrenParent.ID)
		assert.NoError(t, err)
		assert.Len(t, children, 0)
	})

	t.Run("List children for non-existent parent", func(t *testing.T) {
		// The store method itself doesn't check if parent exists, it just queries.
		// The API handler should ideally check if parent exists first.
		// For the store method, an empty slice is expected if no children match parent_id.
		children, err := testStore.ListChildComponents(99999) // Non-existent parent ID
		assert.NoError(t, err) // Store method itself shouldn't error if parent ID simply has no children
		assert.Len(t, children, 0)
	})
}
