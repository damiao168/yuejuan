-- STORY-A25 keeps the operational copy of a scanner profile on the Windows
-- scan station.  This registry is intentionally limited to non-secret device
-- configuration so a school can later audit/profile its registered stations
-- without uploading scanned pages or device serial numbers.

CREATE TABLE IF NOT EXISTS scanner_profile (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    device_fingerprint VARCHAR(128) NOT NULL,
    name VARCHAR(120) NOT NULL,
    dpi INTEGER NOT NULL CHECK (dpi BETWEEN 150 AND 1200),
    duplex BOOLEAN NOT NULL,
    color_mode VARCHAR(32) NOT NULL CHECK (color_mode IN ('color', 'grayscale', 'black_white')),
    paper_size VARCHAR(32) NOT NULL CHECK (paper_size IN ('A3', 'A4', 'A5', 'Letter', 'Legal')),
    auto_rotate BOOLEAN NOT NULL,
    compression VARCHAR(32) NOT NULL CHECK (compression IN ('jpeg', 'png', 'tiff', 'pdf')),
    template_preset VARCHAR(160) NOT NULL,
    created_by UUID REFERENCES app_user(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, device_fingerprint, name)
);

CREATE INDEX IF NOT EXISTS idx_scanner_profile_tenant_device
    ON scanner_profile(tenant_id, device_fingerprint);
