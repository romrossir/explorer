package api

import (
	"component-service/models"
	"component-service/store"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings" // For parsing URL paths
)

var componentStore = &store.ComponentStore{}

// respondWithError sends a JSON error response.
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON sends a JSON response.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Error marshalling JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// ComponentsHandler routes requests for /components and /components/{id}
func ComponentsHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/") // e.g., ["components", "123"] or ["components"]

	if len(pathParts) == 1 && pathParts[0] == "components" { // /components
		switch r.Method {
		case http.MethodGet:
			listComponents(w, r)
		case http.MethodPost:
			createComponent(w, r)
		default:
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	} else if len(pathParts) == 2 && pathParts[0] == "components" { // /components/{id}
		id, err := strconv.ParseInt(pathParts[1], 10, 64)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid component ID in path")
			return
		}
		switch r.Method {
		case http.MethodGet:
			getComponent(w, r, id)
		case http.MethodPut:
			updateComponent(w, r, id)
		case http.MethodDelete:
			deleteComponent(w, r, id)
		default:
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	} else if len(pathParts) == 3 && pathParts[0] == "components" && pathParts[2] == "children" { // /components/{id}/children
		parentID, err := strconv.ParseInt(pathParts[1], 10, 64)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid parent component ID in path")
			return
		}
		if r.Method == http.MethodGet {
			listChildComponents(w, r, parentID)
		} else {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed for child components endpoint")
		}
	} else {
		respondWithError(w, http.StatusNotFound, "Not found")
	}
}

func createComponent(w http.ResponseWriter, r *http.Request) {
	var comp models.Component
	if err := json.NewDecoder(r.Body).Decode(&comp); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	// Basic validation
	if comp.Name == "" {
		respondWithError(w, http.StatusBadRequest, "Component name is required")
		return
	}

	// If ParentID is present in JSON but is 0, it means "no parent".
	// If ParentID is not in JSON, comp.ParentID.Valid will be false.
	// The store layer handles sql.NullInt64 conversion.

	id, err := componentStore.CreateComponent(&comp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating component: "+err.Error())
		return
	}
	comp.ID = id
	// To return the full component including timestamps, we could fetch it again,
	// but for now, let's return what we have plus the ID.
	// For a more complete response, you might do:
	// createdComp, err := componentStore.GetComponentByID(id)
	// if err != nil { ... }
	// respondWithJSON(w, http.StatusCreated, createdComp)

	// Set CreatedAt and UpdatedAt to empty strings, as they are not returned by CreateComponent
	// but are present in the struct. The actual values are set in the DB.
	// A fetch after create would populate these accurately.
	comp.CreatedAt = ""
	comp.UpdatedAt = ""

	respondWithJSON(w, http.StatusCreated, comp)
}

func getComponent(w http.ResponseWriter, r *http.Request, id int64) {
	comp, err := componentStore.GetComponentByID(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Error getting component: "+err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, comp)
}

func updateComponent(w http.ResponseWriter, r *http.Request, id int64) {
	var comp models.Component
	if err := json.NewDecoder(r.Body).Decode(&comp); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if comp.Name == "" { // Basic validation
		respondWithError(w, http.StatusBadRequest, "Component name is required for update")
		return
	}

	// Ensure the ID from the path is used, not from the body if present.
	err := componentStore.UpdateComponent(id, &comp)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Error updating component: "+err.Error())
		}
		return
	}
	// To return the updated component, fetch it again.
	updatedComp, err := componentStore.GetComponentByID(id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error fetching updated component: "+err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, updatedComp)
}

func deleteComponent(w http.ResponseWriter, r *http.Request, id int64) {
	err := componentStore.DeleteComponent(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, err.Error())
		} else {
			respondWithError(w, http.StatusInternalServerError, "Error deleting component: "+err.Error())
		}
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Component deleted successfully"})
}

func listComponents(w http.ResponseWriter, r *http.Request) {
	comps, err := componentStore.ListComponents()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error listing components: "+err.Error())
		return
	}
	if comps == nil { // Ensure we return an empty list, not null, if no components
		comps = []*models.Component{}
	}
	respondWithJSON(w, http.StatusOK, comps)
}

func listChildComponents(w http.ResponseWriter, r *http.Request, parentID int64) {
	// First, check if the parent component exists
	_, err := componentStore.GetComponentByID(parentID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondWithError(w, http.StatusNotFound, fmt.Sprintf("Parent component with ID %d not found", parentID))
		} else {
			respondWithError(w, http.StatusInternalServerError, "Error checking parent component: "+err.Error())
		}
		return
	}

	children, err := componentStore.ListChildComponents(parentID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error listing child components: "+err.Error())
		return
	}
	if children == nil { // Ensure empty list, not null
		children = []*models.Component{}
	}
	respondWithJSON(w, http.StatusOK, children)
}
