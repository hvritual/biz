-- Branding shares the existing tenant CAS; it does not create a parallel tenant identity.
ALTER TABLE biz_tenants
    ADD COLUMN brand_preset VARCHAR(16) NOT NULL DEFAULT 'blue',
    ADD COLUMN brand_primary VARCHAR(7) NOT NULL DEFAULT '';
