DO $$ BEGIN
    CREATE TYPE proposal_status AS ENUM (
        'Created',
        'Published',
        'Canceled',
        'Approved',
        'Rejected'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version INT,
    tender_id UUID REFERENCES tenders(id) ON DELETE CASCADE,
    author_user_id UUID REFERENCES employee(id) ON DELETE CASCADE,
    author_organization_id UUID REFERENCES organization(id) ON DELETE CASCADE,
    status proposal_status,
    name VARCHAR(100),
    description VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS proposals_versions (
    id UUID REFERENCES proposals(id) ON DELETE CASCADE,
    version INT,
    tender_id UUID REFERENCES tenders(id) ON DELETE CASCADE,
    author_user_id UUID REFERENCES employee(id) ON DELETE CASCADE,
    author_organization_id UUID REFERENCES organization(id) ON DELETE CASCADE,
    status proposal_status,
    name VARCHAR(100),
    description VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS proposal_reviews (
    proposal_id UUID REFERENCES proposals(id) ON DELETE CASCADE,
    user_id UUID REFERENCES employee(id) ON DELETE CASCADE,
    text VARCHAR(1000),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(proposal_id, user_id)
);
