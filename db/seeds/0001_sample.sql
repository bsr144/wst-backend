INSERT INTO households (id, owner_name, address) VALUES
    ('11111111-1111-1111-1111-111111111111', 'Budi Santoso', 'Jl. Merdeka No. 1, Jakarta'),
    ('22222222-2222-2222-2222-222222222222', 'Siti Rahayu', 'Jl. Diponegoro No. 45, Bandung')
ON CONFLICT (id) DO NOTHING;

INSERT INTO waste_pickups (id, household_id, type, status, safety_check) VALUES
    ('33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111', 'organic', 'pending', false),
    ('44444444-4444-4444-4444-444444444444', '11111111-1111-1111-1111-111111111111', 'electronic', 'pending', true)
ON CONFLICT (id) DO NOTHING;
