package cache

import (
	"component-service/models"
	"component-service/store" // Import the store package
	"database/sql"
	"fmt"
	"sync"
)

// ComponentCache holds the in-memory cache for components.
type ComponentCache struct {
	mu                 sync.RWMutex
	componentsByID     map[int64]*models.Component
	childrenByParentID map[int64][]*models.Component // Key is ParentID.Value.Int64, or a special key for nil parents
	allComponents      []*models.Component
}

var GlobalComponentCache *ComponentCache

// RootParentIDKey is a conventional key for root components (those with no parent or ParentID.Valid is false).
const RootParentIDKey = 0 // Or use -1 if 0 is a valid component ID and also a valid ParentID for some components

func NewComponentCache() *ComponentCache {
	return &ComponentCache{
		componentsByID:     make(map[int64]*models.Component),
		childrenByParentID: make(map[int64][]*models.Component),
		allComponents:      make([]*models.Component, 0),
	}
}

// InitGlobalCache initializes and populates the global component cache.
// It fetches all components from the store and organizes them for quick access.
func InitGlobalCache(s store.ComponentStoreInterface) error {
	GlobalComponentCache = NewComponentCache() // Initialize the global instance

	GlobalComponentCache.mu.Lock()
	defer GlobalComponentCache.mu.Unlock()

	components, err := s.ListComponents()
	if err != nil {
		return fmt.Errorf("failed to list components for cache initialization: %w", err)
	}

	// Temporary maps for building cache structure efficiently
	tempComponentsByID := make(map[int64]*models.Component)
	tempChildrenByParentID := make(map[int64][]*models.Component)
	var tempAllComponents []*models.Component

	for _, component := range components {
		// Deep copy the component to avoid a pointer to loop variable issue
		// and to ensure cache has its own copy if components from store are modified elsewhere.
		// However, models.Component is a struct of basic types and sql.NullInt64,
		// so a direct copy is fine unless there are deeper pointers. For now, direct assign is okay.
		compCopy := *component // Create a copy

		tempComponentsByID[compCopy.ID] = &compCopy
		tempAllComponents = append(tempAllComponents, &compCopy)

		var parentKey int64
		if compCopy.ParentID.Valid {
			parentKey = compCopy.ParentID.Int64
		} else {
			parentKey = RootParentIDKey // Use the defined key for root elements
		}
		tempChildrenByParentID[parentKey] = append(tempChildrenByParentID[parentKey], &compCopy)
	}

	GlobalComponentCache.componentsByID = tempComponentsByID
	GlobalComponentCache.childrenByParentID = tempChildrenByParentID
	GlobalComponentCache.allComponents = tempAllComponents

	// fmt.Printf("Cache initialized with %d components, %d parent groups.\n", len(GlobalComponentCache.allComponents), len(GlobalComponentCache.childrenByParentID))
	return nil
}

// Set adds or updates a component in the cache.
// It handles updating all relevant internal maps and slices.
func (c *ComponentCache) Set(component *models.Component) {
	if component == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove from old parent's children list if it exists and parent has changed
	if oldComp, exists := c.componentsByID[component.ID]; exists {
		if oldComp.ParentID != component.ParentID { // This comparison works for sql.NullInt64
			oldParentKey := getParentKey(oldComp.ParentID)
			c.removeChildFromParent(oldComp.ID, oldParentKey)
		}
	}

	compCopy := *component // Store a copy
	c.componentsByID[compCopy.ID] = &compCopy

	// Update allComponents: remove old if exists, then add new
	// More efficient to rebuild if component found, or append if not.
	foundInAll := false
	for i, comp := range c.allComponents {
		if comp.ID == compCopy.ID {
			c.allComponents[i] = &compCopy // replace with new version
			foundInAll = true
			break
		}
	}
	if !foundInAll {
		c.allComponents = append(c.allComponents, &compCopy) // add if new
	}

	// Add to new parent's children list
	newParentKey := getParentKey(compCopy.ParentID)
	// Ensure the component is not duplicated in the children list of the new parent
	// This check is important if Set can be called multiple times with the same component instance
	// or if a component is moved to a parent list where it might already exist due to some complex scenario.
	// First, try to remove it from the new parent's list to avoid duplicates, then add it.
	c.removeChildFromParent(compCopy.ID, newParentKey)
	c.childrenByParentID[newParentKey] = append(c.childrenByParentID[newParentKey], &compCopy)
}

// Delete removes a component from the cache.
func (c *ComponentCache) Delete(componentID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	component, exists := c.componentsByID[componentID]
	if !exists {
		return // Not in cache
	}

	delete(c.componentsByID, componentID)

	var updatedAllComponents []*models.Component
	for _, comp := range c.allComponents {
		if comp.ID != componentID {
			updatedAllComponents = append(updatedAllComponents, comp)
		}
	}
	c.allComponents = updatedAllComponents

	parentKey := getParentKey(component.ParentID)
	c.removeChildFromParent(componentID, parentKey)
}

// removeChildFromParent is an internal helper to remove a child from a parent's list.
// Assumes lock is already held.
func (c *ComponentCache) removeChildFromParent(childID int64, parentKey int64) {
	children, ok := c.childrenByParentID[parentKey]
	if !ok {
		return
	}
	var updatedChildren []*models.Component
	for _, child := range children {
		if child.ID != childID {
			updatedChildren = append(updatedChildren, child)
		}
	}
	if len(updatedChildren) == 0 {
		delete(c.childrenByParentID, parentKey) // Clean up map if parent has no more children
	} else if len(updatedChildren) < len(children) { // Only update if something actually changed
		c.childrenByParentID[parentKey] = updatedChildren
	}
}

// GetByID retrieves a component by its ID from the cache.
func (c *ComponentCache) GetByID(id int64) (*models.Component, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	component, found := c.componentsByID[id]
	if !found {
		return nil, false
	}
	compCopy := *component // Return a copy
	return &compCopy, true
}

// GetAll retrieves all components from the cache.
func (c *ComponentCache) GetAll() []*models.Component {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// Return copies to prevent external modification of cached objects
	copiedComponents := make([]*models.Component, 0, len(c.allComponents))
	for _, comp := range c.allComponents {
		compCopy := *comp
		copiedComponents = append(copiedComponents, &compCopy)
	}
	return copiedComponents
}

// GetChildren retrieves direct children of a given parent ID from the cache.
// The parentID parameter here is the actual value of the parent's ID, or RootParentIDKey for root items.
func (c *ComponentCache) GetChildren(parentID int64) ([]*models.Component, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// The key in childrenByParentID (parentKey) was determined by getParentKey during Set/Init.
	// So, the parentID passed to this function should match that key.
	children, found := c.childrenByParentID[parentID]

	if !found || len(children) == 0 {
		return []*models.Component{}, false // Return empty slice and false if no children or parent key not found
	}

	copiedChildren := make([]*models.Component, 0, len(children))
	for _, comp := range children {
		compCopy := *comp
		copiedChildren = append(copiedChildren, &compCopy)
	}
	return copiedChildren, true
}

// getParentKey is a helper to determine the key for the childrenByParentID map.
// It uses RootParentIDKey if ParentID is not valid (i.e., for root components).
func getParentKey(parentID sql.NullInt64) int64 {
	if parentID.Valid {
		return parentID.Int64
	}
	return RootParentIDKey
}
