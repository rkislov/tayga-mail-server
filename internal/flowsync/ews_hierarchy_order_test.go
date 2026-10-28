package flowsync

import (
	"strings"
	"testing"
)

func TestEWSHierarchyParentsBeforeChildrenAcrossPages(t *testing.T) {
	nodes := map[string]*protocolNode{}
	current := map[string]string{}
	for id, parent := range map[string]string{"z-parent": "root", "a-child": "z-parent", "0-grandchild": "a-child", "b-sibling": "root"} {
		node, err := parseProtocolXML([]byte(`<Folder><FolderId Id="` + id + `"/><ParentFolderId Id="` + parent + `"/></Folder>`))
		if err != nil {
			t.Fatal(err)
		}
		nodes[id] = node
		current[id] = id
	}
	previous := map[string]string{}
	for page := 0; page < 4; page++ {
		ids, next, more := diffEWSFolders(previous, current, nodes, 1)
		if len(ids) != 1 {
			t.Fatalf("page %d: %v", page, ids)
		}
		parent := extractAttr(nodes[ids[0]].render(), "ParentFolderId", "Id")
		if _, exists := current[parent]; exists && previous[parent] == "" {
			t.Fatalf("child %s before parent %s", ids[0], parent)
		}
		if more != (page < 3) {
			t.Fatalf("wrong continuation: %d %v", page, more)
		}
		previous = next
	}
	ids, _, more := diffEWSFolders(previous, current, nodes, 1)
	if len(ids) != 0 || more {
		t.Fatal("unchanged folders re-emitted")
	}
}

func TestEWSWellKnownFoldersIgnoreNestedDuplicateNames(t *testing.T) {
	ctx, st, u, eas, _, _ := syncFixture(t)
	h := &ewsHandler{store: st, ms: eas.ms}
	st.EnsureMailbox(ctx, u.ID, "Archive", "")
	st.EnsureMailbox(ctx, u.ID, "Archive/INBOX", "")
	if err := h.ensureStandardMailFolders(ctx, u); err != nil {
		t.Fatal(err)
	}
	inbox, err := st.GetMailbox(ctx, u.ID, "INBOX")
	if err != nil {
		t.Fatal(err)
	}
	response, err := h.getFolder(ctx, u, `<GetFolder><FolderIds><DistinguishedFolderId Id="inbox"/></FolderIds></GetFolder>`)
	if err != nil || !strings.Contains(response, `FolderId Id="`+inbox.ID+`"`) {
		t.Fatalf("wrong inbox: %s %v", response, err)
	}
}
