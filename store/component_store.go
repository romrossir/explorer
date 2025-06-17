package store

import (
	"component-service/cache"
	"component-service/db"
	"component-service/models"
	"database/sql"
	"fmt"
	"time"
)

// ComponentStoreInterface defines the methods that the cache will use to interact with the component store.
type ComponentStoreInterface interface {
	ListComponents() ([]*models.Component, error)
}

// ComponentStore handles database operations for components.
type ComponentStore struct{}

// CreateComponent adds a new component to the database and updates the cache.
func (s *ComponentStore) CreateComponent(component *models.Component) (int64, error) {
	dbConn := db.GetDB()
	query := `INSERT INTO components (name, description, parent_id, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var parentID sql.NullInt64
	if component.ParentID.Valid && component.ParentID.Int64 != 0 {
		parentID = component.ParentID
	}
	var id int64
	err := dbConn.QueryRow(
		query,
		component.Name,
		component.Description,
		parentID,
		time.Now(),
		time.Now(),
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("error creating component: %w", err)
	}

	if cache.GlobalComponentCache != nil {
		createdComponent := &models.Component{}
		var createdAt, updatedAt time.Time
		// Direct DB query to get the component as it was created, including DB-set fields
		errScan := dbConn.QueryRow("SELECT id, name, description, parent_id, created_at, updated_at FROM components WHERE id = $1", id).Scan(
			&createdComponent.ID, &createdComponent.Name, &createdComponent.Description, &createdComponent.ParentID, &createdAt, &updatedAt,
		)
		if errScan == nil {
			createdComponent.CreatedAt = createdAt.Format(time.RFC3339)
			createdComponent.UpdatedAt = updatedAt.Format(time.RFC3339)
			cache.GlobalComponentCache.Set(createdComponent)
		} else {
			// Log error: failed to fetch created component for cache update. Non-fatal for the create operation itself.
			fmt.Printf("Error fetching component %d for cache update after create: %v\n", id, errScan)
		}
	}
	return id, nil
}

// GetComponentByID retrieves a component by its ID.
// It checks the global cache first if initialized.
func (s *ComponentStore) GetComponentByID(id int64) (*models.Component, error) {
	if cache.GlobalComponentCache != nil {
		if component, found := cache.GlobalComponentCache.GetByID(id); found {
			return component, nil
		}
		// If cache is initialized and component is not found, it means it does not exist according to the cache.
		return nil, fmt.Errorf("component with ID %d not found", id)
	}

	// Fallback to database if cache is not initialized
	dbConn := db.GetDB()
	query := "SELECT id, name, description, parent_id, created_at, updated_at FROM components WHERE id = $1"
	row := dbConn.QueryRow(query, id)
	component := &models.Component{}
	var createdAtDb, updatedAtDb time.Time

	err := row.Scan(
		&component.ID,
		&component.Name,
		&component.Description,
		&component.ParentID,
		&createdAtDb,
		&updatedAtDb,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("component with ID %d not found", id)
		}
		return nil, fmt.Errorf("error getting component by ID %d: %w", id, err)
	}
	component.CreatedAt = createdAtDb.Format(time.RFC3339)
	component.UpdatedAt = updatedAtDb.Format(time.RFC3339)
	return component, nil
}

// UpdateComponent updates an existing component in the database and invalidates cache.
func (s *ComponentStore) UpdateComponent(id int64, component *models.Component) error {
	dbConn := db.GetDB()
	query := "UPDATE components SET name = $1, description = $2, parent_id = $3, updated_at = $4 WHERE id = $5"
	var parentID sql.NullInt64
	if component.ParentID.Valid && component.ParentID.Int64 != 0 {
		parentID = component.ParentID
	}

	result, err := dbConn.Exec(
		query,
		component.Name,
		component.Description,
		parentID,
		time.Now(), // Set UpdatedAt
		id,
	)
	if err != nil {
		return fmt.Errorf("error updating component with ID %d: %w", id, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected for update on component ID %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("component with ID %d not found for update", id)
	}

	if cache.GlobalComponentCache != nil {
		updatedComponent := &models.Component{}
		var createdAt, updatedAt time.Time
		// Direct DB query to get the updated component, including new UpdatedAt
		errScan := dbConn.QueryRow("SELECT id, name, description, parent_id, created_at, updated_at FROM components WHERE id = $1", id).Scan(
			&updatedComponent.ID, &updatedComponent.Name, &updatedComponent.Description, &updatedComponent.ParentID, &createdAt, &updatedAt,
		)
		if errScan == nil {
			updatedComponent.CreatedAt = createdAt.Format(time.RFC3339)
			updatedComponent.UpdatedAt = updatedAt.Format(time.RFC3339)
			cache.GlobalComponentCache.Set(updatedComponent)
		} else {
			// Log error: failed to fetch updated component for cache update. Non-fatal.
			fmt.Printf("Error fetching component %d for cache update after update: %v\n", id, errScan)
		}
	}
	return nil
}

// DeleteComponent removes a component from the database and invalidates cache.
func (s *ComponentStore) DeleteComponent(id int64) error {
	dbConn := db.GetDB()
	query := "DELETE FROM components WHERE id = $1"
	result, err := dbConn.Exec(query, id)
	if err != nil {
		return fmt.Errorf("error deleting component with ID %d: %w", id, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error getting rows affected for delete on component ID %d: %w", id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("component with ID %d not found for deletion", id)
	}

	if cache.GlobalComponentCache != nil {
		cache.GlobalComponentCache.Delete(id)
	}
	return nil
}

// ListComponents retrieves all components.
// It uses the cache if initialized.
func (s *ComponentStore) ListComponents() ([]*models.Component, error) {
	if cache.GlobalComponentCache != nil {
		return cache.GlobalComponentCache.GetAll(), nil
	}

	// Fallback to database if cache is not initialized
	dbConn := db.GetDB()
	query := "SELECT id, name, description, parent_id, created_at, updated_at FROM components ORDER BY created_at DESC"
	rows, err := dbConn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error listing components: %w", err)
	}
	defer rows.Close()
	var components []*models.Component
	for rows.Next() {
		component_model := &models.Component{}
		var createdAtDb, updatedAtDb time.Time
		err_scan := rows.Scan(
			&component_model.ID,
			&component_model.Name,
			&component_model.Description,
			&component_model.ParentID,
			&createdAtDb,
			&updatedAtDb,
		)
		if err_scan != nil {
			return nil, fmt.Errorf("error scanning component row: %w", err_scan)
		}
		component_model.CreatedAt = createdAtDb.Format(time.RFC3339)
		component_model.UpdatedAt = updatedAtDb.Format(time.RFC3339)
		components = append(components, component_model)
	}
	if err_rows := rows.Err(); err_rows != nil {
		return nil, fmt.Errorf("error iterating component rows: %w", err_rows)
	}
	return components, nil
}

// ListChildComponents retrieves all direct children of a given parent component ID.
// It uses the cache if initialized.
func (s *ComponentStore) ListChildComponents(parentID int64) ([]*models.Component, error) {
	if cache.GlobalComponentCache != nil {
		children, _ := cache.GlobalComponentCache.GetChildren(parentID)
		return children, nil
	}

	// Fallback to database if cache is not initialized
	dbConn := db.GetDB()
	query := "SELECT id, name, description, parent_id, created_at, updated_at FROM components WHERE parent_id = $1 ORDER BY created_at ASC"
	rows, err := dbConn.Query(query, parentID)
	if err != nil {
		return nil, fmt.Errorf("error listing child components for parent ID %d: %w", parentID, err)
	}
	defer rows.Close()
	var components []*models.Component
	for rows.Next() {
		component_model := &models.Component{}
		var createdAtDb, updatedAtDb time.Time
		err_scan := rows.Scan(
			&component_model.ID,
			&component_model.Name,
			&component_model.Description,
			&component_model.ParentID,
			&createdAtDb,
			&updatedAtDb,
		)
		if err_scan != nil {
			return nil, fmt.Errorf("error scanning child component row: %w", err_scan)
		}
		component_model.CreatedAt = createdAtDb.Format(time.RFC3339)
		component_model.UpdatedAt = updatedAtDb.Format(time.RFC3339)
		components = append(components, component_model)
	}
	if err_rows := rows.Err(); err_rows != nil {
		return nil, fmt.Errorf("error iterating child component rows for parent ID %d: %w", err_rows)
	}
	return components, nil
}
