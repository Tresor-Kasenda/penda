package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"penda/framework/orm"
	fwtest "penda/framework/testing"
)

func newTestApp(t *testing.T) *fwtest.Client {
	t.Helper()

	dsn := fmt.Sprintf("file:rest-api-test-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := orm.Open(orm.Config{Dialector: "sqlite", DSN: dsn, MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	server, err := BuildApp(db)
	if err != nil {
		t.Fatalf("build app: %v", err)
	}

	return fwtest.NewClient(server)
}

func TestUserCRUD(t *testing.T) {
	client := newTestApp(t)

	createResp := client.PostJSON("/api/users", map[string]string{
		"name":  "Ada",
		"email": "ada@example.com",
	})
	fwtest.AssertStatus(t, createResp, http.StatusCreated)

	var created User
	if err := createResp.DecodeJSON(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("expected created ID, got %+v", created)
	}

	listResp := client.Get("/api/users")
	fwtest.AssertStatus(t, listResp, http.StatusOK)
	var users []User
	if err := listResp.DecodeJSON(&users); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(users) != 1 || users[0].Email != "ada@example.com" {
		t.Fatalf("unexpected users list: %+v", users)
	}

	getResp := client.Get(fmt.Sprintf("/api/users/%d", created.ID))
	fwtest.AssertStatus(t, getResp, http.StatusOK)
	var fetched User
	if err := getResp.DecodeJSON(&fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.Name != "Ada" {
		t.Fatalf("unexpected fetched user: %+v", fetched)
	}

	// PATCH using low-level Do to keep client minimal.
	patchReqBody := []byte(`{"name":"Ada Lovelace"}`)
	patchReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/users/%d", created.ID), bytes.NewReader(patchReqBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp := client.Do(patchReq)
	fwtest.AssertStatus(t, patchResp, http.StatusOK)

	var updated User
	if err := patchResp.DecodeJSON(&updated); err != nil {
		t.Fatalf("decode patch response: %v", err)
	}
	if updated.Name != "Ada Lovelace" {
		t.Fatalf("unexpected updated user: %+v", updated)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/users/%d", created.ID), nil)
	deleteResp := client.Do(deleteReq)
	fwtest.AssertStatus(t, deleteResp, http.StatusNoContent)

	missingResp := client.Get(fmt.Sprintf("/api/users/%d", created.ID))
	fwtest.AssertStatus(t, missingResp, http.StatusNotFound)
}

func TestCreateUserValidation(t *testing.T) {
	client := newTestApp(t)

	resp := client.PostJSON("/api/users", map[string]string{"name": ""})
	fwtest.AssertStatus(t, resp, http.StatusBadRequest)
}
