# Component Service

This is a Go-based backend service for managing hierarchical components. It provides a REST API for CRUD operations and persists component data in a PostgreSQL database.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Setup](#setup)
  - [Environment Variables](#environment-variables)
  - [Database Setup](#database-setup)
- [Running the Service](#running-the-service)
- [API Endpoints](#api-endpoints)
  - [Component Model](#component-model)
  - [Create Component](#create-component)
  - [Get Component by ID](#get-component-by-id)
  - [Update Component](#update-component)
  - [Delete Component](#delete-component)
  - [List All Components](#list-all-components)
  - [List Child Components](#list-child-components)
- [Building from Source](#building-from-source)
- [Running Tests (TODO)](#running-tests-todo)

## Prerequisites

- Go (version 1.18 or later recommended)
- PostgreSQL (running and accessible)
- Git

## Setup

1.  **Clone the repository:**
    ```bash
    git clone <repository-url>
    cd component-service
    ```

2.  **Install dependencies:**
    The necessary Go packages (currently `github.com/lib/pq`) will be fetched automatically when you build or run the service if you have Go modules enabled. You can also explicitly fetch them:
    ```bash
    go mod tidy
    # or
    go get
    ```

### Environment Variables

The service requires the following environment variables to be set for database connection:

-   `DB_HOST`: Hostname of the PostgreSQL server (e.g., `localhost`)
-   `DB_PORT`: Port of the PostgreSQL server (e.g., `5432`)
-   `DB_USER`: PostgreSQL username
-   `DB_PASSWORD`: PostgreSQL password
-   `DB_NAME`: Name of the database to use
-   `DB_SSLMODE`: SSL mode for connection (e.g., `disable`, `require`). Defaults to `disable` if not set.

Optionally, you can set the `PORT` environment variable to specify the port on which the service will listen (defaults to `8080`).

You can set these in your shell, or use a `.env` file (though this project doesn't include a `.env` loader by default, you can add one like `github.com/joho/godotenv`).

Example:
```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=youruser
export DB_PASSWORD=yoursecret
export DB_NAME=components_db
export PORT=8080
```

### Database Setup

1.  Ensure your PostgreSQL server is running.
2.  Create the database specified by `DB_NAME`.
3.  Apply the schema:
    Connect to your PostgreSQL database (e.g., using `psql`) and execute the contents of `db/schema.sql`:
    ```bash
    psql -U youruser -d components_db -a -f db/schema.sql
    ```
    This will create the `components` table, an index, and a trigger for updating timestamps.

## Running the Service

Once the environment variables are set and the database is configured, you can run the service:

```bash
go run main.go
```

The service will start, and you should see log messages indicating database initialization and the server starting, typically on port 8080.

## API Endpoints

The base URL for the API is `http://localhost:<PORT>`.

### Component Model

```json
{
    "id": 1,
    "name": "Component Name",
    "description": "Detailed description of the component.",
    "parent_id": null, // or integer ID of the parent component
    "created_at": "2023-10-27T10:00:00Z", // RFC3339 format
    "updated_at": "2023-10-27T10:05:00Z"  // RFC3339 format
}
```
- `parent_id`: If `null`, the component is a root component.

### Create Component

-   **Endpoint:** `POST /components/`
-   **Request Body:** Component object (ID, CreatedAt, UpdatedAt are ignored). `parent_id` is optional.
    ```json
    {
        "name": "New Component",
        "description": "This is a new component.",
        "parent_id": 1 // Optional: ID of the parent component
    }
    ```
-   **Response:** `201 Created` with the created component object (ID will be populated).
    ```json
    {
        "id": 2,
        "name": "New Component",
        "description": "This is a new component.",
        "parent_id": { "Int64": 1, "Valid": true } // Example if parent_id was provided
    }
    ```
    *(Note: The `CreatedAt` and `UpdatedAt` fields in the immediate response from POST might be empty strings. A subsequent GET will show the DB-generated timestamps.)*


### Get Component by ID

-   **Endpoint:** `GET /components/{id}`
-   **Response:** `200 OK` with the component object or `404 Not Found`.

### Update Component

-   **Endpoint:** `PUT /components/{id}`
-   **Request Body:** Component object with fields to update.
    ```json
    {
        "name": "Updated Component Name",
        "description": "Updated description.",
        "parent_id": null // Example: making it a root component
    }
    ```
-   **Response:** `200 OK` with the updated component object or `404 Not Found`.

### Delete Component

-   **Endpoint:** `DELETE /components/{id}`
-   **Response:** `200 OK` with a success message or `404 Not Found`.
    ```json
    {
        "message": "Component deleted successfully"
    }
    ```

### List All Components

-   **Endpoint:** `GET /components/`
-   **Response:** `200 OK` with an array of component objects.
    ```json
    [
        { "id": 1, ... },
        { "id": 2, ... }
    ]
    ```

### List Child Components

-   **Endpoint:** `GET /components/{id}/children`
-   **Response:** `200 OK` with an array of direct child component objects or `404 Not Found` if the parent component doesn't exist.
    ```json
    [
        { "id": 3, "parent_id": {"Int64": <id>, "Valid": true }, ... }
    ]
    ```

## Building from Source

To build an executable:

```bash
go build -o component-service-app main.go
```
This will create an executable named `component-service-app` in the current directory.

## Running Tests (TODO)

(Instructions for running tests will be added here once tests are implemented.)
