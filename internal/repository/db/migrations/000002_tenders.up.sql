DO $$ BEGIN
    CREATE TYPE tender_status AS ENUM (
        'Created',
        'Published',
        'Closed'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE tender_service_type AS ENUM (
        'Construction',
        'Delivery',
        'Manufacture'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS tenders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    version INT,
    organization_id UUID REFERENCES organization(id) ON DELETE CASCADE,
    author_id UUID REFERENCES employee(id) ON DELETE CASCADE,
    status tender_status,
    service_type tender_service_type,
    name VARCHAR(100) UNIQUE,
    description VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tenders_versions (
    id UUID REFERENCES tenders(id) ON DELETE CASCADE,
    version INT,
    organization_id UUID REFERENCES organization(id) ON DELETE CASCADE,
    author_id UUID REFERENCES employee(id) ON DELETE CASCADE,
    status tender_status,
    service_type tender_service_type,
    name VARCHAR(100),
    description VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
