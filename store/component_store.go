package store

import (
	"component-service/db"
	"component-service/models"
	"database/sql"
	"fmt"
	"time"
)

// ComponentStore handles database operations for components.
type ComponentStore struct{}

// CreateComponent adds a new component to the database.
func (s *ComponentStore) CreateComponent(component *models.Component) (int64, error) {
	dbConn := db.GetDB()
	query := `INSERT INTO components (name, description, parent_id, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5) RETURNING id`

	// Ensure ParentID is correctly handled (nil for root components)
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
		time.Now(), // Set CreatedAt
		time.Now(), // Set UpdatedAt
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("error creating component: %w", err)
	}
	return id, nil
}

// GetComponentByID retrieves a component by its ID.
func (s *ComponentStore) GetComponentByID(id int64) (*models.Component, error) {
	dbConn := db.GetDB()
	query := `SELECT id, name, description, parent_id, created_at, updated_at
              FROM components WHERE id = $1`

	row := dbConn.QueryRow(query, id)
	component := &models.Component{}
	var createdAt, updatedAt time.Time // Use time.Time for scanning

	err := row.Scan(
		&component.ID,
		&component.Name,
		&component.Description,
		&component.ParentID,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("component with ID %d not found", id)
		}
		return nil, fmt.Errorf("error getting component by ID %d: %w", id, err)
	}
	component.CreatedAt = createdAt.Format(time.RFC3339)
	component.UpdatedAt = updatedAt.Format(time.RFC3339)
	return component, nil
}

// UpdateComponent updates an existing component in the database.
func (s *ComponentStore) UpdateComponent(id int64, component *models.Component) error {
	dbConn := db.GetDB()
	query := `UPDATE components
              SET name = $1, description = $2, parent_id = $3, updated_at = $4
              WHERE id = $5`

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
	return nil
}

// DeleteComponent removes a component from the database by its ID.
func (s *ComponentStore) DeleteComponent(id int64) error {
	dbConn := db.GetDB()
	query := `DELETE FROM components WHERE id = $1`

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
	return nil
}

// ListComponents retrieves all components.
// TODO: Add pagination and filtering options.
func (s *ComponentStore) ListComponents() ([]*models.Component, error) {
	dbConn := db.GetDB()
	query := `SELECT id, name, description, parent_id, created_at, updated_at
              FROM components ORDER BY created_at DESC` // Example ordering

	rows, err := dbConn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error listing components: %w", err)
	}
	defer rows.Close()

	var components []*models.Component
	for rows.Next() {
		component := &models.Component{}
		var createdAt, updatedAt time.Time
		err := rows.Scan(
			&component.ID,
			&component.Name,
			&component.Description,
			&component.ParentID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning component row: %w", err)
		}
		component.CreatedAt = createdAt.Format(time.RFC3339)
		component.UpdatedAt = updatedAt.Format(time.RFC3339)
		components = append(components, component)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating component rows: %w", err)
	}

	return components, nil
}

// ListChildComponents retrieves all direct children of a given parent component ID.
func (s *ComponentStore) ListChildComponents(parentID int64) ([]*models.Component, error) {
	dbConn := db.GetDB()
	query := `SELECT id, name, description, parent_id, created_at, updated_at
              FROM components WHERE parent_id = $1 ORDER BY created_at ASC`

	rows, err := dbConn.Query(query, parentID)
	if err != nil {
		return nil, fmt.Errorf("error listing child components for parent ID %d: %w", parentID, err)
	}
	defer rows.Close()

	var components []*models.Component
	for rows.Next() {
		component := &models.Component{}
		var createdAt, updatedAt time.Time
		err := rows.Scan(
			&component.ID,
			&component.Name,
			&component.Description,
			&component.ParentID,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning child component row: %w", err)
		}
		component.CreatedAt = createdAt.Format(time.RFC3339)
		component.UpdatedAt = updatedAt.Format(time.RFC3339)
		components = append(components, component)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating child component rows for parent ID %d: %w", parentID, err)
	}

	return components, nil
}
