ALTER TABLE user_preferences
    ADD COLUMN page_size INT NOT NULL DEFAULT 12;

UPDATE user_preferences
SET page_size = projects_page_size;

ALTER TABLE user_preferences
    DROP COLUMN projects_page_size,
    DROP COLUMN tasks_page_size;
