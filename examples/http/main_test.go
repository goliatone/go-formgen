package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRelationshipDatasetAuthorRecordFetchAndUpdate(t *testing.T) {
	mux := newRelationshipTestMux()
	authorID := "11111111-1111-4111-8111-111111111111"

	response := performRelationshipRequest(mux, http.MethodGet, "/api/authors/"+authorID, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET author status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var initial authorRecord
	decodeRelationshipResponse(t, response, &initial)
	if initial.FullName != "Ada Lovelace" {
		t.Fatalf("initial author name = %q, want %q", initial.FullName, "Ada Lovelace")
	}

	payload := []byte(`{"full_name":"Ada Lovelace Updated","email":"ada.updated@example.com","active":true}`)
	response = performRelationshipRequest(mux, http.MethodPut, "/api/authors/"+authorID, payload)
	if response.Code != http.StatusOK {
		t.Fatalf("PUT author status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var updated authorRecord
	decodeRelationshipResponse(t, response, &updated)
	if updated.ID != authorID || updated.FullName != "Ada Lovelace Updated" || updated.Email != "ada.updated@example.com" {
		t.Fatalf("updated author = %#v", updated)
	}

	response = performRelationshipRequest(mux, http.MethodPut, "/api/authors/"+authorID, []byte(`{"full_name":"","email":""}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid author update status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	response = performRelationshipRequest(mux, http.MethodGet, "/api/authors/missing", nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing author status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRelationshipDatasetPublisherRecordFetchAndUpdate(t *testing.T) {
	mux := newRelationshipTestMux()
	publisherID := "cccc3333-cccc-4333-8333-cccccccccccc"

	response := performRelationshipRequest(mux, http.MethodGet, "/api/publishing-houses/"+publisherID, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("GET publisher status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var initial publishingHouseRecord
	decodeRelationshipResponse(t, response, &initial)
	if initial.Name != "Northwind Publishing" {
		t.Fatalf("initial publisher name = %q, want %q", initial.Name, "Northwind Publishing")
	}

	payload := []byte(`{"name":"Northwind Books","imprint_prefix":"NWB"}`)
	response = performRelationshipRequest(mux, http.MethodPut, "/api/publishing-houses/"+publisherID, payload)
	if response.Code != http.StatusOK {
		t.Fatalf("PUT publisher status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var updated publishingHouseRecord
	decodeRelationshipResponse(t, response, &updated)
	if updated.ID != publisherID || updated.Name != "Northwind Books" || updated.ImprintPrefix != "NWB" {
		t.Fatalf("updated publisher = %#v", updated)
	}

	response = performRelationshipRequest(mux, http.MethodPut, "/api/publishing-houses/"+publisherID, []byte(`{"name":""}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid publisher update status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	response = performRelationshipRequest(mux, http.MethodGet, "/api/publishing-houses/missing", nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing publisher status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func newRelationshipTestMux() *http.ServeMux {
	dataset := newRelationshipDataset()
	mux := http.NewServeMux()
	dataset.register(mux)
	return mux
}

func performRelationshipRequest(mux http.Handler, method, target string, body []byte) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func decodeRelationshipResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
