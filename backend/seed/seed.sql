-- Seed data for TaskFlow
-- Test user: test@example.com / password123

INSERT INTO users (id, name, email, password, created_at)
VALUES (
    'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    'Test User',
    'test@example.com',
    '$2b$12$RDGKSPkugms1FG1LGcXnUuVMFOpbNKer9iij2hrU/ftCTRLinSXVW',
    now()
) ON CONFLICT (email) DO NOTHING;

INSERT INTO projects (id, name, description, owner_id, created_at)
VALUES (
    'b2c3d4e5-f6a7-8901-bcde-f12345678901',
    'Website Redesign',
    'Q2 project to redesign the company website with a modern look and improved UX.',
    'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
    now()
) ON CONFLICT DO NOTHING;

INSERT INTO tasks (id, title, description, status, priority, project_id, assignee_id, created_by, due_date, created_at, updated_at)
VALUES
    (
        'c3d4e5f6-a7b8-9012-cdef-123456789012',
        'Design homepage mockup',
        'Create wireframes and high-fidelity mockups for the new homepage layout.',
        'todo',
        'high',
        'b2c3d4e5-f6a7-8901-bcde-f12345678901',
        'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
        'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
        '2026-04-20',
        now(),
        now()
    ),
    (
        'd4e5f6a7-b8c9-0123-defa-234567890123',
        'Implement responsive navigation',
        'Build a mobile-first responsive navigation bar with hamburger menu.',
        'in_progress',
        'medium',
        'b2c3d4e5-f6a7-8901-bcde-f12345678901',
        'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
        'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
        '2026-04-25',
        now(),
        now()
    ),
    (
        'e5f6a7b8-c9d0-1234-efab-345678901234',
        'Set up CI/CD pipeline',
        'Configure GitHub Actions for automated testing and deployment.',
        'done',
        'low',
        'b2c3d4e5-f6a7-8901-bcde-f12345678901',
        NULL,
        'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
        '2026-04-15',
        now(),
        now()
    )
ON CONFLICT DO NOTHING;
