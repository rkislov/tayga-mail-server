package config

import (
	"path/filepath"
	"testing"
)

func TestMattermostAttachmentPathSurvivesBackendSwitch(t *testing.T) {
	c := Default()
	c.Storage.Driver = "sqlite"
	c.Storage.SQLite.Path = "/srv/tayga/data/tayga.db"
	c.Mailstore.Root = "/srv/tayga/mail"
	want := filepath.Join("/srv/tayga/data", ".mattermost-attachments")
	if c.MattermostAttachmentsPath() != want {
		t.Fatal("SQLite default changed")
	}
	c.Mattermost.AttachmentDirectory = c.MattermostAttachmentsPath()
	c.Storage.Driver = "postgres"
	if c.MattermostAttachmentsPath() != want {
		t.Fatal("attachments moved on backend switch")
	}
}
