package db

import (
	"bytes"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	authTokenEnv = "TURSO_API_TOKEN"
	apiHostEnv   = "TURSO_API_URL"
	groupEnv     = "TURSO_GROUP"
	orgSlugEnv   = "TURSO_ORGANIZATION"
	appNameEnv   = "TURSO_APP_NAME"
)

type Factory struct {
	cache     map[string]*Queries
	orgSlug   string
	apiHost   string
	c         *http.Client
	group     string
	authToken string
	schema    []string
	appName   string
}

func NewFactory(schemaPath string) *Factory {
	// Read db schema
	f, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Fatalf("failed to read sql schema wiht %s", err)
	}

	// Get environment variables
	var missing []string
	authToken := os.Getenv(authTokenEnv)
	if authToken == "" {
		missing = append(missing, authTokenEnv)
	}
	apiHost := os.Getenv(apiHostEnv)
	if apiHost == "" {
		missing = append(missing, apiHostEnv)
	}
	group := os.Getenv(groupEnv)
	if group == "" {
		missing = append(missing, groupEnv)
	}
	orgSlug := os.Getenv(orgSlugEnv)
	if orgSlug == "" {
		missing = append(missing, orgSlugEnv)
	}
	appName := os.Getenv(appNameEnv)
	if appName == "" {
		missing = append(missing, appNameEnv)
	}
	if len(missing) > 0 {
		log.Fatalf("missing environment variables: %s", missing)
	}

	return &Factory{
		cache:     make(map[string]*Queries),
		orgSlug:   orgSlug,
		apiHost:   apiHost,
		c:         &http.Client{},
		group:     group,
		authToken: authToken,
		schema:    strings.Split(string(f), ";"),
		appName:   appName,
	}
}

func (f *Factory) Get(id string) (*Queries, error) {
	// Search cache
	q, ok := f.cache[id]
	if ok {
		return q, nil
	}

	// Try to get already existing
	dbName := fmt.Sprintf("%s-%s", f.appName, strings.ToLower(strings.Replace(id, "user_", "", 1)))
	slog.Info("trying to get db from API", "dbName", dbName)
	url := fmt.Sprintf("%s/v1/organizations/%s/databases/%s", f.apiHost, f.orgSlug, dbName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init db retrieve request with %w", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.authToken))
	resp, err := f.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send retrieve request with %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	default:
		return nil, fmt.Errorf("unexpected response status code %d", resp.StatusCode)

	case http.StatusOK:
		// Deserialize response
		var info retrieveDbResp
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body with %w", err)
		}
		err = json.Unmarshal(body, &info)
		slog.Info("unmarshalled", "info", info)
		if err != nil {
			return nil, fmt.Errorf("failed to deserialzie retrieve response with %w", err)
		}
		// Initialize connection
		token, err := f.createDbToken(dbName)
		if err != nil {
			return nil, fmt.Errorf("failed to create db token with %w", err)
		}
		conn, err := f.initConnection(info.Database.Hostname, token)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to existing database with %w", err)
		}
		q := New(conn)
		slog.Info("db retrieved from API")
		// Update cache
		f.cache[id] = q
		return q, nil

	case http.StatusNotFound:
		// Create new one
		url := fmt.Sprintf("%s/v1/organizations/%s/databases", f.apiHost, f.orgSlug)
		body := []byte(fmt.Sprintf(`{"name": "%s", "group": "%s"}`, dbName, f.group))
		req, err := http.NewRequest(
			"POST",
			url,
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to init db retrieve request with %w", err)
		}
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", f.authToken))
		resp, err := f.c.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send retrieve request with %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		default:
			return nil, fmt.Errorf("unexpected response status code %d", resp.StatusCode)
		case http.StatusConflict:
			// Database was somehow created
			return nil, fmt.Errorf("different thread created db")

		case http.StatusBadRequest:
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response body with %w", err)
			}
			return nil, fmt.Errorf("bad request while db creation: %s", body)

		case http.StatusOK:
			// Deserialize response
			var info retrieveDbResp
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read response body with %w", err)
			}
			err = json.Unmarshal(body, &info)
			if err != nil {
				return nil, fmt.Errorf("failed to deserialize created db response with %w", err)
			}

			// Init connection
			token, err := f.createDbToken(dbName)
			if err != nil {
				return nil, fmt.Errorf("failed to create db token with %w", err)
			}
			conn, err := f.initConnection(info.Database.Hostname, token)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to existing database with %w", err)
			}

			// Apply schema
			var errs []error
			for _, ddl := range f.schema {
				_, err := conn.Exec(ddl)
				if err != nil {
					errs = append(errs, err)
				}
			}
			if len(errs) > 0 {
				return nil, fmt.Errorf("failed to apply schema with %w", errors.Join(errs...))
			}

			q := New(conn)
			slog.Info("new db was created")
			// Update cache
			f.cache[id] = q
			return q, nil
		}
	}
}

func (f Factory) initConnection(hostname, token string) (*sql.DB, error) {
	url := fmt.Sprintf("libsql://%s?authToken=%s", hostname, token)
	return sql.Open("libsql", url)
}

func (f Factory) createDbToken(dbName string) (string, error) {
	slog.Info("creating new db token for", "dbName", dbName)
	baseURL := fmt.Sprintf("%s/v1/organizations/%s/databases/%s/auth/tokens", f.apiHost, f.orgSlug, dbName)
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}
	q := u.Query()
	q.Set("expiration", "2w")
	q.Set("authorization", "full-access")
	u.RawQuery = q.Encode()

	slog.Info("quering", "url", u.String())
	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+f.authToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var tokenResponse tursoTokenResp
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return tokenResponse.JWT, nil
}

type retrieveDbResp struct {
	Database struct {
		Hostname string
	} `json:"database"`
}

type tursoTokenResp struct {
	JWT string `json:"jwt"`
}
