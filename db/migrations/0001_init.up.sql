CREATE TABLE households (
    id          uuid PRIMARY KEY,
    owner_name  text NOT NULL,
    address     text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE waste_pickups (
    id           uuid PRIMARY KEY,
    household_id uuid NOT NULL REFERENCES households(id) ON DELETE RESTRICT,
    type         text NOT NULL CHECK (type IN ('organic','plastic','paper','electronic')),
    status       text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','scheduled','completed','canceled')),
    pickup_date  timestamptz,
    safety_check boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_pickups_household_status ON waste_pickups (household_id, status);
CREATE INDEX idx_pickups_sweep ON waste_pickups (created_at)
    WHERE type = 'organic' AND status IN ('pending','scheduled');

CREATE TABLE payments (
    id             uuid PRIMARY KEY,
    household_id   uuid NOT NULL REFERENCES households(id) ON DELETE RESTRICT,
    waste_id       uuid NOT NULL REFERENCES waste_pickups(id) ON DELETE RESTRICT,
    amount         numeric(14,2) NOT NULL,
    payment_date   timestamptz,
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','failed')),
    proof_file_url text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_payments_waste ON payments (waste_id);
CREATE INDEX idx_payments_household_status ON payments (household_id, status);
CREATE INDEX idx_payments_date ON payments (payment_date);
