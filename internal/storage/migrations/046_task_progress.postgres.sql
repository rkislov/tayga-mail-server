-- +goose Up
ALTER TABLE tasks ADD COLUMN status TEXT NOT NULL DEFAULT 'NotStarted' CHECK (status IN ('NotStarted','InProgress','Completed','WaitingOnOthers','Deferred'));
ALTER TABLE tasks ADD COLUMN percent_complete DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (percent_complete >= 0 AND percent_complete <= 100);
UPDATE tasks SET status='Completed',percent_complete=100 WHERE completed=TRUE;

-- +goose Down
ALTER TABLE tasks DROP COLUMN percent_complete;
ALTER TABLE tasks DROP COLUMN status;
