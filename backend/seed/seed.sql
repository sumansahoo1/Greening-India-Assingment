-- Seed data for TaskFlow
-- Test user: test@example.com / password123

-- Create (or keep) a stable seed user.
INSERT INTO users (id, name, email, password, created_at)
VALUES (
    'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    'Test User',
    'test@example.com',
    '$2b$12$RDGKSPkugms1FG1LGcXnUuVMFOpbNKer9iij2hrU/ftCTRLinSXVW',
    now()
) ON CONFLICT (email) DO NOTHING;

-- Second dev user: andy@example.com / password123
INSERT INTO users (id, name, email, password, created_at)
VALUES (
    'f1e2d3c4-b5a6-7980-9abc-def012345678',
    'Andy',
    'andy@example.com',
    '$2b$12$RDGKSPkugms1FG1LGcXnUuVMFOpbNKer9iij2hrU/ftCTRLinSXVW',
    now()
) ON CONFLICT (email) DO NOTHING;

-- Seed 15 projects and 20 tasks per project (300 tasks total).
-- Each project gets a distinct created_at offset so ORDER BY created_at is stable
-- across restarts (gs=1 is oldest, gs=15 is newest → newest-first list shows gs=15 first).
WITH inserted_projects AS (
    INSERT INTO projects (name, description, owner_id, created_at)
    SELECT
        CASE
            WHEN gs = 1 THEN 'Website Redesign'
            ELSE 'Seed Project ' || lpad(gs::text, 2, '0')
        END,
        CASE
            WHEN gs = 1 THEN 'Q2 project to redesign the company website with a modern look and improved UX.'
            ELSE 'Seeded project #' || gs::text
        END,
        'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
        now() - ((16 - gs) * interval '1 minute')
    FROM generate_series(1, 15) AS gs
    RETURNING id, name
),
numbered_projects AS (
    SELECT
        id,
        row_number() OVER (ORDER BY name) AS project_no
    FROM inserted_projects
)
INSERT INTO tasks (title, description, status, priority, project_id, assignee_id, created_by, due_date, created_at, updated_at)
SELECT
    'Task ' || lpad(t::text, 2, '0') || ' (Project ' || lpad(p.project_no::text, 2, '0') || ')',
    'Seeded task #' || t::text || ' for project #' || p.project_no::text,
    (CASE (t % 3)
        WHEN 1 THEN 'todo'
        WHEN 2 THEN 'in_progress'
        ELSE 'done'
    END)::task_status,
    (CASE (t % 3)
        WHEN 1 THEN 'low'
        WHEN 2 THEN 'medium'
        ELSE 'high'
    END)::task_priority,
    p.id,
    CASE
        WHEN (t % 5) = 0 THEN NULL
        WHEN (t % 2) = 0 THEN 'f1e2d3c4-b5a6-7980-9abc-def012345678'
        ELSE 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'
    END::uuid,
    'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    (current_date + (((p.project_no - 1) * 2 + t)::int))::date,
    now() - (((15 - p.project_no) * 20 + (21 - t)) * interval '1 second'),
    now() - (((15 - p.project_no) * 20 + (21 - t)) * interval '1 second')
FROM numbered_projects p
CROSS JOIN generate_series(1, 20) AS t;
