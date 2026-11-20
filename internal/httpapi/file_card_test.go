package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testFileCard struct {
	CanManage bool   `json:"can_manage"`
	Rights    string `json:"effective_rights"`
	Access    []struct {
		Path, Email, Rights string
		Inherited           bool
	} `json:"access"`
	Shares []struct {
		ID, URL           string
		Inherited, Active bool
	} `json:"shares"`
	Events []struct {
		ID, Action, Subject string
		Actor               fileActor
		ActorEmail          string `json:"actor_email"`
	} `json:"events"`
	Next string `json:"next_cursor"`
}

func readFileCard(t *testing.T, f *fileFixture, user int, path string) testFileCard {
	t.Helper()
	var card testFileCard
	if err := json.Unmarshal(f.call(user, "GET", "/api/v1/files/card?"+path, "", 200).Body.Bytes(), &card); err != nil {
		t.Fatal(err)
	}
	return card
}
func TestFileCardAccessLinksAndIsolation(t *testing.T) {
	f := newFileFixture(t)
	owner := f.users[0]
	f.call(0, "PUT", "/api/v1/files/content?path=team/report.txt", "one", 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team","email":"reader@example.com","rights":"read"}`, 200)
	f.call(0, "POST", "/api/v1/files/access", `{"path":"team/report.txt","email":"reader@example.com","rights":"write"}`, 200)
	f.call(0, "POST", "/api/v1/files/shares", `{"path":"team","password":"secret-password","ttl_hours":24}`, 201)
	link := f.call(0, "POST", "/api/v1/files/shares", `{"path":"team/report.txt","max_downloads":1}`, 201)
	var shared struct{ ID, Token string }
	json.Unmarshal(link.Body.Bytes(), &shared)
	card := readFileCard(t, f, 0, "path=team/report.txt")
	if !card.CanManage || card.Rights != "owner" || len(card.Access) != 2 || len(card.Shares) != 2 {
		t.Fatalf("card %+v", card)
	}
	if len(card.Events) != 3 || card.Events[0].Action != "link_created" {
		t.Fatalf("events %+v", card.Events)
	}
	if card.Events[0].Actor.ID != owner || card.Events[0].ActorEmail != "owner@example.com" {
		t.Fatalf("actor %+v", card.Events[0])
	}
	received := readFileCard(t, f, 1, "path=team/report.txt&owner_id="+owner)
	if received.CanManage || received.Rights != "write" || len(received.Access) != 2 || len(received.Events) != 0 || len(received.Shares) != 0 {
		t.Fatalf("recipient leaked data %+v", received)
	}
	f.call(2, "GET", "/api/v1/files/card?path=team/report.txt&owner_id="+owner, "", 403)
	f.call(-1, "GET", "/api/v1/files/card?path=team/report.txt", "", 401)
	f.call(0, "GET", "/api/v1/files/card?path=../outside", "", 400)
	f.call(0, "GET", "/api/v1/files/card?path=x&owner_id=../../outside", "", 400)
	// A child revoke leaves inherited access effective.
	f.call(0, "DELETE", "/api/v1/files/access", `{"path":"team/report.txt","grantee_id":"`+f.users[1]+`"}`, 200)
	received = readFileCard(t, f, 1, "path=team/report.txt&owner_id="+owner)
	if received.Rights != "read" || len(received.Access) != 1 || !received.Access[0].Inherited {
		t.Fatalf("inherited %+v", received)
	}
	f.call(1, "PUT", "/api/v1/files/content?path=team/report.txt&owner_id="+owner, "denied", 403)
	// Link counters and active state are reflected in the card.
	f.call(-1, "GET", "/s/"+shared.Token, "", 200)
	card = readFileCard(t, f, 0, "path=team/report.txt")
	for _, s := range card.Shares {
		if s.ID == shared.ID && s.Active {
			t.Fatal("exhausted link still active")
		}
	}
	f.call(1, "DELETE", "/api/v1/files/shares/"+shared.ID, "", 404)
	f.call(0, "DELETE", "/api/v1/files/shares/"+shared.ID, "", 200)
	card = readFileCard(t, f, 0, "path=team/report.txt")
	if card.Events[0].Action != "link_revoked" || len(card.Shares) != 1 {
		t.Fatalf("revoke %+v", card)
	}
	root, _ := f.s.fileStateRoot(owner, "activity")
	data, err := os.ReadFile(filepath.Join(root, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(shared.Token)) || bytes.Contains(data, []byte("secret-password")) {
		t.Fatal("secret in activity")
	}
}
func TestFileActivityMoveTrashRestoreAndRecovery(t *testing.T) {
	f := newFileFixture(t)
	owner := f.users[0]
	f.call(0, "PUT", "/api/v1/files/content?path=folder/report.txt", "one", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=folder/report.txt", "two", 200)
	f.call(0, "POST", "/api/v1/files/move", `{"from":"folder","to":"renamed"}`, 200)
	card := readFileCard(t, f, 0, "path=renamed/report.txt")
	if len(card.Events) != 3 || card.Events[0].Action != "moved" {
		t.Fatalf("move %+v", card)
	}
	f.call(0, "PUT", "/api/v1/files/content?path=folder/report.txt", "different", 200)
	fresh := readFileCard(t, f, 0, "path=folder/report.txt")
	if len(fresh.Events) != 1 || fresh.Events[0].Action != "created" {
		t.Fatal("old path inherited history")
	}
	f.call(0, "DELETE", "/api/v1/files?path=renamed", "", 200)
	trash := revisions(t, f.call(0, "GET", "/api/v1/files/library/trash", "", 200))
	f.call(0, "POST", "/api/v1/files/library/restore", `{"kind":"trash","id":"`+trash[0].ID+`"}`, 200)
	card = readFileCard(t, f, 0, "path=renamed/report.txt")
	if len(card.Events) != 5 || card.Events[0].Action != "restored" || card.Events[1].Action != "trashed" {
		t.Fatalf("restore %+v", card)
	}
	// Replay after a server restart, recovering an incomplete last record.
	root, _ := f.s.fileStateRoot(owner, "activity")
	log := filepath.Join(root, "events.jsonl")
	fd, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	fd.WriteString(`{"interrupted":`)
	fd.Close()
	f.s.fileActivities = nil
	card = readFileCard(t, f, 0, "path=renamed/report.txt")
	if len(card.Events) != 5 {
		t.Fatalf("replay %+v", card)
	}
	data, _ := os.ReadFile(log)
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("partial tail not truncated")
	}
	f.call(0, "PUT", "/api/v1/files/content?path=renamed/report.txt", "three", 200)
	card = readFileCard(t, f, 0, "path=renamed/report.txt")
	if len(card.Events) != 6 {
		t.Fatal("append after recovery")
	}
	// Removing an old version must not attach its event to a replacement file.
	oldVersions := revisions(t, f.call(0, "GET", "/api/v1/files/library/versions?path=renamed/report.txt", "", 200))
	f.call(0, "DELETE", "/api/v1/files?path=renamed/report.txt", "", 200)
	f.call(0, "PUT", "/api/v1/files/content?path=renamed/report.txt", "replacement", 200)
	for _, v := range oldVersions {
		f.call(0, "DELETE", "/api/v1/files/library/versions", `{"id":"`+v.ID+`"}`, 200)
	}
	replacementCard := readFileCard(t, f, 0, "path=renamed/report.txt")
	if len(replacementCard.Events) != 1 {
		t.Fatalf("old version event leaked to replacement %+v", replacementCard.Events)
	}

}
func TestFileActivityPaginationAndSources(t *testing.T) {
	f := newFileFixture(t)
	owner := f.users[0]
	f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "one", 200)
	for i := 0; i < 52; i++ {
		f.s.fileMu.Lock()
		f.s.recordFileActivity(owner, "a.txt", "updated", []fileActor{{ID: owner, Source: "webdav"}})
		f.s.fileMu.Unlock()
	}
	first := readFileCard(t, f, 0, "path=a.txt")
	if len(first.Events) != 50 || first.Next == "" {
		t.Fatalf("first %+v", first)
	}
	second := readFileCard(t, f, 0, "path=a.txt&before="+first.Next)
	if len(second.Events) != 3 || second.Next != "" {
		t.Fatalf("second %+v", second)
	}
	if first.Events[0].Actor.Source != "webdav" {
		t.Fatal("wrong source")
	}
	seen := map[string]bool{}
	for _, e := range first.Events {
		seen[e.ID] = true
	}
	for _, e := range second.Events {
		if seen[e.ID] {
			t.Fatal("duplicate page")
		}
	}
	f.call(0, "POST", "/api/v1/files/access", `{"path":"a.txt","email":"editor@example.com","rights":"write"}`, 200)
	f.call(2, "PUT", "/api/v1/files/content?path=a.txt&owner_id="+owner, "editor change", 200)
	card := readFileCard(t, f, 0, "path=a.txt")
	if card.Events[0].Actor.ID != f.users[2] {
		t.Fatalf("wrong writer %+v", card.Events[0])
	}
}

func TestFileCardRevocationDuringUpload(t *testing.T) {
	for _, public := range []bool{false, true} {
		t.Run(map[bool]string{false: "user", true: "link"}[public], func(t *testing.T) {
			f := newFileFixture(t)
			f.call(0, "PUT", "/api/v1/files/content?path=a.txt", "original", 200)
			url := "/api/v1/files/content?path=a.txt&owner_id=" + f.users[0]
			linkID := ""
			if public {
				var link struct{ ID, Token string }
				json.Unmarshal(f.call(0, "POST", "/api/v1/files/shares", `{"path":"a.txt","rights":"write"}`, 201).Body.Bytes(), &link)
				url = "/s/" + link.Token
				linkID = link.ID
			} else {
				f.call(0, "POST", "/api/v1/files/access", `{"path":"a.txt","email":"editor@example.com","rights":"write"}`, 200)
			}
			started, resume := make(chan struct{}), make(chan struct{})
			reader := &pausedReader{started: started, resume: resume, data: strings.NewReader("slow upload")}
			req := httptest.NewRequest("PUT", url, reader)
			if !public {
				req.Header.Set("Authorization", "Bearer "+f.tokens[2])
			}
			response := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { f.s.Handler().ServeHTTP(response, req); close(done) }()
			<-started
			if public {
				f.call(0, "DELETE", "/api/v1/files/shares/"+linkID, "", 200)
			} else {
				f.call(0, "DELETE", "/api/v1/files/access", `{"path":"a.txt","grantee_id":"`+f.users[2]+`"}`, 200)
			}
			close(resume)
			<-done
			if response.Code != 403 {
				t.Fatalf("revoked upload: %d %s", response.Code, response.Body.String())
			}
			if got := f.call(0, "GET", "/api/v1/files/content?path=a.txt", "", 200).Body.String(); got != "original" {
				t.Fatalf("overwritten: %s", got)
			}
			card := readFileCard(t, f, 0, "path=a.txt")
			for _, e := range card.Events {
				if e.Action == "updated" {
					t.Fatal("failed upload logged as success")
				}
			}
		})
	}
}
