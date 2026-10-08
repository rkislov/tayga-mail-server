ALTER TABLE calendars ADD COLUMN color TEXT NOT NULL DEFAULT '#c77c35';
ALTER TABLE calendars ADD COLUMN public_token TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX calendars_public_token ON calendars(public_token) WHERE public_token <> '';
