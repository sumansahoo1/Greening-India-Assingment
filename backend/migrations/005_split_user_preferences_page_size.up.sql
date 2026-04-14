ALTER TABLE user_preferences
    ADD COLUMN projects_page_size INT NOT NULL DEFAULT 12,
    ADD COLUMN tasks_page_size    INT NOT NULL DEFAULT 12;

UPDATE user_preferences
SET
    projects_page_size = page_size,
    tasks_page_size    = page_size;

ALTER TABLE user_preferences
    DROP COLUMN page_size;
