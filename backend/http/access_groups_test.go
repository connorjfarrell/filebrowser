package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// groupCtx builds a request context for a user; admin controls the Admin permission.
func groupCtx(username string, admin bool) *requestContext {
	u := &users.User{Username: username}
	u.Permissions.Admin = admin
	return &requestContext{user: u}
}

// doGroupRequest runs handler with the given body and returns the status, recorder and handler error.
func doGroupRequest(t *testing.T, handler func(http.ResponseWriter, *http.Request, *requestContext) (int, error), method, target, body string, ctx *requestContext) (int, *httptest.ResponseRecorder, error) {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	status, err := handler(rec, req, ctx)
	return status, rec, err
}

// assertJSONMessage checks the response body is valid JSON carrying the given message.
func assertJSONMessage(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response body is not JSON (%q): %v", rec.Body.String(), err)
	}
	if got["message"] != want {
		t.Fatalf("expected message %q, got %q", want, got["message"])
	}
}

func TestGroupPutHandler_RequiresAdmin(t *testing.T) {
	setupTestEnv(t)
	status, _, _ := doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", `{"group":"g","members":[]}`, groupCtx("bob", false))
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", status)
	}
	if len(store.Access.GetAllGroups()) != 0 {
		t.Fatal("non-admin request must not create a group")
	}
}

func TestGroupPutHandler_Validation(t *testing.T) {
	setupTestEnv(t)
	ctx := groupCtx("admin", true)
	for name, body := range map[string]string{
		"invalid json": `{not json`,
		"empty name":   `{"group":"","members":["a"]}`,
		"blank name":   `{"group":"   ","members":["a"]}`,
	} {
		status, _, _ := doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", body, ctx)
		if status != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", name, status)
		}
	}
	if len(store.Access.GetAllGroups()) != 0 {
		t.Fatal("invalid requests must not create groups")
	}
}

func TestGroupPutHandler_ReplacesMembersAndReturnsJSON(t *testing.T) {
	setupTestEnv(t)
	ctx := groupCtx("admin", true)

	status, rec, err := doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", `{"group":" editors ","members":["alice","bob"]}`, ctx)
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "group saved")

	if _, _, err = doGroupRequest(t, groupPutHandler, http.MethodPut, "/api/access/group", `{"group":"editors","members":["carol"]}`, ctx); err != nil {
		t.Fatal(err)
	}
	got := store.Access.GetGroupMembers()["editors"]
	if len(got) != 1 || got[0] != "carol" {
		t.Fatalf("expected membership replaced with [carol], got %v", got)
	}
}

func TestGroupDeleteHandler_MemberVersusWholeGroup(t *testing.T) {
	setupTestEnv(t)
	ctx := groupCtx("admin", true)
	if err := store.Access.SetGroupMembers("editors", []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}

	status, rec, err := doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group?group=editors&user=alice", "", ctx)
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "user removed from group")
	if got := store.Access.GetGroupMembers()["editors"]; len(got) != 1 || got[0] != "bob" {
		t.Fatalf("expected only bob left, got %v", got)
	}

	status, rec, err = doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group?group=editors", "", ctx)
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "group deleted")
	if _, ok := store.Access.GetGroupMembers()["editors"]; ok {
		t.Fatal("group should be deleted when no user is given")
	}

	status, _, _ = doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group", "", ctx)
	if status != http.StatusBadRequest {
		t.Fatalf("expected 400 without group, got %d", status)
	}
	status, _, _ = doGroupRequest(t, groupDeleteHandler, http.MethodDelete, "/api/access/group?group=x", "", groupCtx("bob", false))
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", status)
	}
}

func TestGroupPostHandler_ReturnsJSON(t *testing.T) {
	setupTestEnv(t)
	status, rec, err := doGroupRequest(t, groupPostHandler, http.MethodPost, "/api/access/group?group=editors&user=alice", "", groupCtx("admin", true))
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected 200, got %d (err=%v)", status, err)
	}
	assertJSONMessage(t, rec, "user added to group")
	if got := store.Access.GetUserGroups("alice"); len(got) != 1 || got[0] != "editors" {
		t.Fatalf("expected alice in editors, got %v", got)
	}
}

func TestGroupGetHandler_MembersParam(t *testing.T) {
	setupTestEnv(t)
	ctx := groupCtx("admin", true)
	if err := store.Access.SetGroupMembers("editors", []string{"alice"}); err != nil {
		t.Fatal(err)
	}

	decode := func(target string) GroupListResponse {
		t.Helper()
		status, rec, err := doGroupRequest(t, groupGetHandler, http.MethodGet, target, "", ctx)
		if status != http.StatusOK || err != nil {
			t.Fatalf("expected 200, got %d (err=%v)", status, err)
		}
		var resp GroupListResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}

	plain := decode("/api/access/groups")
	if len(plain.Groups) != 1 || plain.Members != nil {
		t.Fatalf("expected names only without members=true, got %+v", plain)
	}
	withMembers := decode("/api/access/groups?members=true")
	if got := withMembers.Members["editors"]; len(got) != 1 || got[0] != "alice" {
		t.Fatalf("expected members[editors]=[alice], got %+v", withMembers)
	}
}
