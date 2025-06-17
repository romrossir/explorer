package cache

import (
	"component-service/models"
	"component-service/store"
	"database/sql"
	"reflect"
	"sort"
	"testing"
)

// MockComponentStore is a mock implementation of ComponentStoreInterface for testing.
type MockComponentStore struct {
	mockComponents      []*models.Component
	ListComponentsError error
}

// ListComponents implements the ComponentStoreInterface for MockComponentStore.
func (m *MockComponentStore) ListComponents() ([]*models.Component, error) {
	if m.ListComponentsError != nil {
		return nil, m.ListComponentsError
	}
	// Return copies to mimic real store behavior
	componentsCopy := make([]*models.Component, len(m.mockComponents))
	for i, comp := range m.mockComponents {
		c := *comp
		componentsCopy[i] = &c
	}
	return componentsCopy, nil
}

// Helper to create a valid sql.NullInt64
func nullInt64(val int64) sql.NullInt64 {
	return sql.NullInt64{Int64: val, Valid: true}
}

// Helper to create an invalid (null) sql.NullInt64
func invalidNullInt64() sql.NullInt64 {
    return sql.NullInt64{Valid: false}
}

// Sample components defined globally for use in multiple tests
var (
	comp1Global = &models.Component{ID: 1, Name: "Comp 1", ParentID: invalidNullInt64()}
	comp2Global = &models.Component{ID: 2, Name: "Comp 2", ParentID: nullInt64(1)}
	comp3Global = &models.Component{ID: 3, Name: "Comp 3", ParentID: nullInt64(1)}
	comp4Global = &models.Component{ID: 4, Name: "Comp 4", ParentID: invalidNullInt64()}
	comp5Global = &models.Component{ID: 5, Name: "Comp 5", ParentID: nullInt64(4)}
    comp6Global = &models.Component{ID: 6, Name: "Comp 6", ParentID: nullInt64(2)}
)

// TestInitGlobalCache uses table-driven tests for various scenarios.
func TestInitGlobalCache(t *testing.T) {
	// Use copies of global components for test data to prevent modification across tests
	c1 := *comp1Global; c2 := *comp2Global; c3 := *comp3Global;
	c4 := *comp4Global; c5 := *comp5Global; c6 := *comp6Global

	tests := []struct {
		name              string
		initialComponents []*models.Component
		expectedTotal     int
		expectedChildren  map[int64]int
        expectedRoots     int
	}{
		{
			name:              "Typical case with multiple components",
			initialComponents: []*models.Component{&c1, &c2, &c3, &c4, &c5, &c6},
			expectedTotal:     6,
			expectedChildren:  map[int64]int{1: 2, 4: 1, 2: 1},
            expectedRoots:     2,
		},
		{
			name:              "No components",
			initialComponents: []*models.Component{},
			expectedTotal:     0,
			expectedChildren:  map[int64]int{},
            expectedRoots:     0,
		},
		{
			name:              "Only root components",
			initialComponents: []*models.Component{&c1, &c4},
			expectedTotal:     2,
			expectedChildren:  map[int64]int{},
            expectedRoots:     2,
		},
        {
            name: "Single component (root)",
            initialComponents: []*models.Component{&c1},
            expectedTotal: 1,
            expectedChildren: map[int64]int{},
            expectedRoots: 1,
        },
        {
            name: "Single component (child of a non-listed parent)",
            initialComponents: []*models.Component{{ID: 10, Name: "Child of 99", ParentID: nullInt64(99)}},
            expectedTotal: 1,
            expectedChildren: map[int64]int{99:1},
            expectedRoots: 0,
        },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &MockComponentStore{mockComponents: tt.initialComponents}
			// Reset GlobalComponentCache for each test run in InitGlobalCache tests
			GlobalComponentCache = nil
			err := InitGlobalCache(mockStore)
			if err != nil {
				t.Fatalf("InitGlobalCache failed: %v", err)
			}
			if GlobalComponentCache == nil {
				t.Fatal("GlobalComponentCache is nil after InitGlobalCache")
			}

			allCached := GlobalComponentCache.GetAll()
			if len(allCached) != tt.expectedTotal {
				t.Errorf("Expected %d total components, got %d", tt.expectedTotal, len(allCached))
			}

			for _, initialComp := range tt.initialComponents {
				expectedComp := *initialComp
				cachedComp, found := GlobalComponentCache.GetByID(initialComp.ID)
				if !found {
					t.Errorf("Expected component %d to be in cache, but not found", initialComp.ID)
					continue
				}
				// Before comparing, ensure ParentID has same Valid state if Int64 might be 0
				// This is mostly if RootParentIDKey is 0 and a component might have ParentID.Int64 = 0 but Valid = true.
				// However, our getParentKey logic standardizes this. The check is more about deep equality.
				if !reflect.DeepEqual(cachedComp, &expectedComp) {
					t.Errorf("Component %d data mismatch. Expected %+v, Got %+v", initialComp.ID, &expectedComp, cachedComp)
				}
			}

            for parentID, expectedCount := range tt.expectedChildren {
                children, foundChildrenMapEntry := GlobalComponentCache.GetChildren(parentID)
                // If we expect children, the map entry (parent key) should exist.
                // The 'found' from GetChildren refers to whether children were found, not if the parent key exists in childrenByParentID map.
                // Let's refine this check: The cache's GetChildren returns (slice, bool) where bool is true if children are found.
                // So, if expectedCount > 0, foundChildrenMapEntry should be true.
                // If expectedCount == 0, foundChildrenMapEntry could be false (parent key not in map) or true (parent key in map, but slice is empty).
                // The current GetChildren returns `false` if children slice is empty.
                if expectedCount > 0 && !foundChildrenMapEntry {
                     t.Errorf("For parent %d with expected children, GetChildren 'found' was false", parentID)
                }
                if expectedCount == 0 && foundChildrenMapEntry {
                     t.Errorf("For parent %d with no expected children, GetChildren 'found' was true", parentID)
                }

                if len(children) != expectedCount {
                    t.Errorf("For parent %d, expected %d children, got %d", parentID, expectedCount, len(children))
                }
            }

            rootChildrenFromCache, foundRoots := GlobalComponentCache.GetChildren(RootParentIDKey)
            if tt.expectedRoots > 0 && !foundRoots {
                t.Errorf("Expected roots to be found (found=true) when expectedRoots > 0, but found=false")
            }
            if tt.expectedRoots == 0 && foundRoots {
                 t.Errorf("Expected no roots to be found (found=false) when expectedRoots == 0, but found=true")
            }
            if len(rootChildrenFromCache) != tt.expectedRoots {
                 t.Errorf("Expected %d root components via GetChildren(RootParentIDKey), got %d", tt.expectedRoots, len(rootChildrenFromCache))
            }
		})
	}
}

func TestComponentCache_Getters_And_CopySemantics(t *testing.T) {
    c101 := *comp1Global // Use a copy for test data
    c101.ID = 101; c101.Name = "C101"

    c102 := *comp2Global // Use a copy
    c102.ID = 102; c102.Name = "C102"; c102.ParentID = nullInt64(101)

	initialCompsForGetterTest := []*models.Component{&c101, &c102}

	mockStore := &MockComponentStore{mockComponents: initialCompsForGetterTest}
    // Reset GlobalComponentCache before this test block
    GlobalComponentCache = nil
	if err := InitGlobalCache(mockStore); err != nil {
		t.Fatalf("Setup for Getters_And_CopySemantics: InitGlobalCache failed: %v", err)
	}
    if GlobalComponentCache == nil {
        t.Fatal("GlobalComponentCache is nil after InitGlobalCache in Getters_And_CopySemantics setup")
    }

	t.Run("GetByID returns copy", func(t *testing.T) {
		comp, found := GlobalComponentCache.GetByID(c101.ID)
		if !found {
			t.Fatalf("Component %d not found", c101.ID)
		}
		originalName := comp.Name
		comp.Name = "Modified Name by TestGetByID"

		refetchedComp, _ := GlobalComponentCache.GetByID(c101.ID)
		if refetchedComp.Name != originalName {
			t.Errorf("GetByID failed to return a copy. Expected name '%s', got '%s'", originalName, refetchedComp.Name)
		}
	})

	t.Run("GetAll returns copies", func(t *testing.T) {
		allComps := GlobalComponentCache.GetAll()
		if len(allComps) == 0 {
			t.Fatal("GetAll returned no components for copy test")
		}
        var firstCompCopy *models.Component
        originalName := "" // Initialize to avoid potential issues if component not found

        // Find the component in the slice and store its original name
        for _, c := range allComps {
            if c.ID == c101.ID {
                firstCompCopy = c // This is a pointer to a copy from GetAll
                originalName = c.Name
                break
            }
        }

        if firstCompCopy == nil {
            t.Fatalf("Component %d not found in GetAll result", c101.ID)
        }

		firstCompCopy.Name = "Modified Name by TestGetAll" // Modify the copy

		refetchedComp, found := GlobalComponentCache.GetByID(c101.ID) // Get from cache again
        if !found {
             t.Fatalf("Component %d not found by GetByID after GetAll test modification", c101.ID)
        }
		if refetchedComp.Name != originalName {
			t.Errorf("GetAll failed to return copies. Expected name '%s' for ID %d, got '%s'", originalName, c101.ID, refetchedComp.Name)
		}
	})

	t.Run("GetChildren returns copies", func(t *testing.T) {
		children, found := GlobalComponentCache.GetChildren(c101.ID)
		if !found || len(children) == 0 {
			t.Fatalf("GetChildren found no children for parent %d or parent not found", c101.ID)
		}
		childCopy := children[0]
        originalChildName := childCopy.Name
		childCopy.Name = "Modified Name by TestGetChildren"

		refetchedChildren, _ := GlobalComponentCache.GetChildren(c101.ID)
		var refetchedChildToCheck *models.Component
		for _, c := range refetchedChildren {
			if c.ID == childCopy.ID {
				refetchedChildToCheck = c
				break
			}
		}
		if refetchedChildToCheck == nil {
			t.Fatalf("Child with ID %d not found in refetched children list after modification attempt", childCopy.ID)
		}
		if refetchedChildToCheck.Name != originalChildName {
			t.Errorf("GetChildren failed to return copies. Expected name '%s' for child ID %d, got '%s'", originalChildName, childCopy.ID, refetchedChildToCheck.Name)
		}
	})

    t.Run("GetByID non-existent", func(t *testing.T) {
        _, found := GlobalComponentCache.GetByID(9999)
        if found {
            t.Error("GetByID found component 9999 which should not exist")
        }
    })

    t.Run("GetChildren non-existent parent", func(t *testing.T) {
        children, found := GlobalComponentCache.GetChildren(8888)
        if found {
            // This is okay if found is true but children list is empty.
            // The 'found' from GetChildren means "parent key exists and has children".
            // If parent key doesn't exist, or has no children, found is false.
            t.Error("GetChildren 'found' was true for non-existent parent 8888")
        }
        if len(children) != 0 {
             t.Errorf("Expected 0 children for non-existent parent 8888, got %d", len(children))
        }
    })

    t.Run("GetChildren parent with no children", func(t *testing.T){
        // Temporarily add a component that will be a parent but have no children listed under it yet.
        parentNoChildren := &models.Component{ID: 505, Name: "ParentWithNoChildren", ParentID: invalidNullInt64()}
        GlobalComponentCache.Set(parentNoChildren) // Assuming Set works for this test setup

        children, found := GlobalComponentCache.GetChildren(parentNoChildren.ID)
        if found { // If GetChildren's 'found' is true, it means it found children. This should be false.
            t.Errorf("Expected 'found' to be false for parent %d that has no children listed under it, but got true", parentNoChildren.ID)
        }
        if len(children) != 0 {
            t.Errorf("Expected 0 children for parent %d (which has no children), got %d", parentNoChildren.ID, len(children))
        }
        GlobalComponentCache.Delete(parentNoChildren.ID) // Clean up
    })
}

// TODO: Add tests for Set (add new, update existing, change parent) and Delete

// TestComponentCache_Set tests the Set method for adding and updating components.
func TestComponentCache_Set(t *testing.T) {
	baseCompsForSetTest := []*models.Component{
		{ID: 10, Name: "Set_C10", ParentID: invalidNullInt64()},
		{ID: 20, Name: "Set_C20", ParentID: nullInt64(10)},
	}
	mockStoreForSet := &MockComponentStore{mockComponents: baseCompsForSetTest}

	t.Run("Set_AddNewComponent_Root", func(t *testing.T) {
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
		if err := InitGlobalCache(mockStoreForSet); err != nil { t.Fatalf("Init failed: %v", err) }
		newComp := &models.Component{ID: 30, Name: "Set_C30_NewRoot", ParentID: invalidNullInt64()}
		GlobalComponentCache.Set(newComp)

		cached, found := GlobalComponentCache.GetByID(30)
		if !found || cached.Name != newComp.Name {
			t.Errorf("AddNewComponent_Root: component not added or data mismatch")
		}
		if len(GlobalComponentCache.GetAll()) != len(baseCompsForSetTest)+1 {
			t.Errorf("AddNewComponent_Root: incorrect total count after add. Expected %d, got %d", len(baseCompsForSetTest)+1, len(GlobalComponentCache.GetAll()))
		}
		// Recalculate expected roots based on current cache state
		currentRoots := 0
		for _, c := range GlobalComponentCache.GetAll() { if !c.ParentID.Valid { currentRoots++ } }
		if currentRoots != 2 { // Base C10 and New C30
             t.Errorf("AddNewComponent_Root: expected 2 root components, got %d", currentRoots)
        }
	})

	t.Run("Set_AddNewComponent_Child", func(t *testing.T) {
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
		if err := InitGlobalCache(mockStoreForSet); err != nil { t.Fatalf("Init failed: %v", err) }
		newChild := &models.Component{ID: 40, Name: "Set_C40_NewChildOf10", ParentID: nullInt64(10)}
		GlobalComponentCache.Set(newChild)

		cached, found := GlobalComponentCache.GetByID(40)
		if !found || cached.Name != newChild.Name {
			t.Errorf("AddNewComponent_Child: component not added or data mismatch")
		}
		childrenOf10, _ := GlobalComponentCache.GetChildren(10)
		if len(childrenOf10) != 2 { // Base C20 and New C40
			t.Errorf("AddNewComponent_Child: expected 2 children for parent 10, got %d", len(childrenOf10))
		}
	})

	t.Run("Set_UpdateExistingComponent_NameChange", func(t *testing.T) {
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
		if err := InitGlobalCache(mockStoreForSet); err != nil { t.Fatalf("Init failed: %v", err) }
		updatedComp20 := &models.Component{ID: 20, Name: "Set_C20_UpdatedName", ParentID: nullInt64(10)}
		GlobalComponentCache.Set(updatedComp20)

		cached, _ := GlobalComponentCache.GetByID(20)
		if cached.Name != updatedComp20.Name {
			t.Errorf("UpdateExistingComponent_NameChange: name not updated. Expected '%s', got '%s'", updatedComp20.Name, cached.Name)
		}
	})

	t.Run("Set_UpdateExistingComponent_ReParent", func(t *testing.T) {
        reparentComps := []*models.Component{
            {ID: 10, Name: "R_C10", ParentID: invalidNullInt64()},
		    {ID: 20, Name: "R_C20", ParentID: nullInt64(10)},
            {ID: 30, Name: "R_C30_NewParent", ParentID: invalidNullInt64()},
        }
        mockStoreForReparent := &MockComponentStore{mockComponents: reparentComps}
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
		if err := InitGlobalCache(mockStoreForReparent); err != nil { t.Fatalf("Init failed: %v", err) }

		compToReparent := &models.Component{ID: 20, Name: "R_C20_Reparented", ParentID: nullInt64(30)}
		GlobalComponentCache.Set(compToReparent)

		cached, _ := GlobalComponentCache.GetByID(20)
		if !cached.ParentID.Valid || cached.ParentID.Int64 != 30 {
			t.Errorf("ReParent: parent not updated. Expected parent 30, got %v", cached.ParentID)
		}
		childrenOf10, _ := GlobalComponentCache.GetChildren(10)
		if len(childrenOf10) != 0 {
			t.Errorf("ReParent: old parent 10 should have 0 children, got %d", len(childrenOf10))
		}
		childrenOf30, foundNewParentChildren := GlobalComponentCache.GetChildren(30)
		if !foundNewParentChildren || len(childrenOf30) != 1 || childrenOf30[0].ID != 20 {
			t.Errorf("ReParent: new parent 30 should have 1 child (ID 20), got %d children (found=%v, childID=%v)", len(childrenOf30), foundNewParentChildren, childrenOf30[0].ID )
		}
	})

    t.Run("Set_UpdateToRoot", func(t *testing.T){
        updateToRootComps := []*models.Component{
            {ID: 10, Name: "UTR_C10", ParentID: invalidNullInt64()},
		    {ID: 20, Name: "UTR_C20", ParentID: nullInt64(10)},
        }
        mockStoreForUpdateToRoot := &MockComponentStore{mockComponents: updateToRootComps}
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
		if err := InitGlobalCache(mockStoreForUpdateToRoot); err != nil { t.Fatalf("Init failed: %v", err) }

        compToMakeRoot := &models.Component{ID: 20, Name: "UTR_C20_NowRoot", ParentID: invalidNullInt64()}
        GlobalComponentCache.Set(compToMakeRoot)

        cached, _ := GlobalComponentCache.GetByID(20)
        if cached.ParentID.Valid {
            t.Errorf("Set_UpdateToRoot: Expected C20 to be a root, ParentID is %v", cached.ParentID)
        }
        childrenOf10, _ := GlobalComponentCache.GetChildren(10)
        if len(childrenOf10) != 0 {
             t.Errorf("Set_UpdateToRoot: Expected parent 10 to have 0 children, got %d", len(childrenOf10))
        }

        allCompsCurrent := GlobalComponentCache.GetAll()
        currentRootsCount := 0
        for _, c := range allCompsCurrent { if !c.ParentID.Valid { currentRootsCount++ } }
        if currentRootsCount != 2 { // UTR_C10 and UTR_C20_NowRoot
            t.Errorf("Set_UpdateToRoot: Expected 2 root components, got %d", currentRootsCount)
        }
    })
}

// TestComponentCache_Delete tests the Delete method.
func TestComponentCache_Delete(t *testing.T) {
	compsForDeleteTest := []*models.Component{
		{ID: 100, Name: "Del_C100", ParentID: invalidNullInt64()},
		{ID: 200, Name: "Del_C200", ParentID: nullInt64(100)},
		{ID: 300, Name: "Del_C300", ParentID: nullInt64(100)},
		{ID: 400, Name: "Del_C400", ParentID: invalidNullInt64()},
	}
    mockStoreForDelete := &MockComponentStore{mockComponents: compsForDeleteTest}
    initialTotalForDeleteSubtests := len(compsForDeleteTest)

	t.Run("Delete_ExistingComponent_Child", func(t *testing.T) {
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
        if err := InitGlobalCache(mockStoreForDelete); err != nil {t.Fatalf("Init DeleteChild failed: %v", err)}
		GlobalComponentCache.Delete(200)

		_, found := GlobalComponentCache.GetByID(200)
		if found {
			t.Errorf("DeleteChild: component 200 still found after delete")
		}
		if len(GlobalComponentCache.GetAll()) != initialTotalForDeleteSubtests-1 {
			t.Errorf("DeleteChild: incorrect total count. Expected %d, got %d", initialTotalForDeleteSubtests-1, len(GlobalComponentCache.GetAll()))
		}
		childrenOf100, foundChildren := GlobalComponentCache.GetChildren(100)
		if !foundChildren || len(childrenOf100) != 1 || (len(childrenOf100) == 1 && childrenOf100[0].ID != 300) {
			t.Errorf("DeleteChild: parent 100 should have 1 child (300), got %d (found=%v, or wrong child)", len(childrenOf100), foundChildren)
		}
	})

	t.Run("Delete_ExistingComponent_RootWithNoChildren", func(t *testing.T) {
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
        if err := InitGlobalCache(mockStoreForDelete); err != nil {t.Fatalf("Init DeleteRootNoChildren failed: %v", err)}
		GlobalComponentCache.Delete(400)

		_, found := GlobalComponentCache.GetByID(400)
		if found {
			t.Errorf("DeleteRootNoChildren: component 400 still found after delete")
		}
		if len(GlobalComponentCache.GetAll()) != initialTotalForDeleteSubtests-1 {
			t.Errorf("DeleteRootNoChildren: incorrect total count. Expected %d, got %d", initialTotalForDeleteSubtests-1, len(GlobalComponentCache.GetAll()))
		}
	})

    t.Run("Delete_ExistingComponent_RootWithChildren", func(t *testing.T) {
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
        if err := InitGlobalCache(mockStoreForDelete); err != nil {t.Fatalf("Init DeleteRootWithChildren failed: %v", err)}
        GlobalComponentCache.Delete(100)

        _, found := GlobalComponentCache.GetByID(100)
        if found {
            t.Errorf("DeleteRootWithChildren: C100 (ID 100) still found")
        }

        // Children of the deleted root should still exist (now as orphans or attached to RootParentIDKey implicitly)
        // Let's verify their existence and potentially their new parentage if we expect them to become roots.
        c200, foundC200 := GlobalComponentCache.GetByID(200)
        c300, foundC300 := GlobalComponentCache.GetByID(300)

        if !foundC200 {
            t.Errorf("DeleteRootWithChildren: Child C200 not found, it should remain")
        } else if c200.ParentID.Valid && c200.ParentID.Int64 == 100 {
            // This depends on whether Delete re-parents children of deleted components to RootParentIDKey or leaves ParentID as is.
            // Current ComponentCache.Delete does NOT re-parent. It only removes the target component.
            // So, their ParentID will still point to the now-deleted 100.
            // The childrenByParentID map for key 100 will be gone because C100 is gone.
            // This is an important aspect to clarify in cache behavior or test.
            // For now, we just check they exist.
            // If the design implies children become roots, then this test needs adjustment:
            // if c200.ParentID.Valid { t.Errorf("C200 should be a root, but ParentID is %v", c200.ParentID) }
        }

        if !foundC300 {
            t.Errorf("DeleteRootWithChildren: Child C300 not found, it should remain")
        } // Similar check for C300's parentage if needed.

        // The childrenByParentID entry for the deleted parent (100) should be gone.
        childrenOfDeleted100, foundEntryFor100 := GlobalComponentCache.childrenByParentID[100]
        if foundEntryFor100 {
             t.Errorf("DeleteRootWithChildren: childrenByParentID map still has entry for deleted parent 100, contains %d children", len(childrenOfDeleted100))
        }


        if len(GlobalComponentCache.GetAll()) != initialTotalForDeleteSubtests-1 { // Only C100 is removed
             t.Errorf("DeleteRootWithChildren: Expected %d components after deleting C100, got %d", initialTotalForDeleteSubtests-1, len(GlobalComponentCache.GetAll()))
        }
    })

	t.Run("Delete_NonExistingComponent", func(t *testing.T) {
		// Reset GlobalComponentCache for this sub-test
		GlobalComponentCache = nil
        if err := InitGlobalCache(mockStoreForDelete); err != nil {t.Fatalf("Init DeleteNonExisting failed: %v", err)}
		currentCount := len(GlobalComponentCache.GetAll())
		GlobalComponentCache.Delete(999)
		if len(GlobalComponentCache.GetAll()) != currentCount {
			t.Errorf("Delete_NonExistingComponent: count changed after deleting non-existent component")
		}
	})
}
