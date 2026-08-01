-- +goose Up
-- +goose StatementBegin
-- UUID v7 default for entity PKs (PostgreSQL 16 has no native uuidv7()).
CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS uuid
LANGUAGE plpgsql
VOLATILE
PARALLEL SAFE
AS $$
DECLARE
  unix_ts_ms bytea;
  uuid_bytes bytea;
BEGIN
  unix_ts_ms := substring(int8send(floor(extract(epoch from clock_timestamp()) * 1000)::bigint) from 3);
  uuid_bytes := unix_ts_ms || substring(uuid_send(gen_random_uuid()) from 7 for 10);
  -- version 7
  uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 15) | 112);
  -- RFC 4122 variant
  uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 63) | 128);
  RETURN encode(uuid_bytes, 'hex')::uuid;
END
$$;

ALTER TABLE alert_rules ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE alerts ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE request_reply_probes ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE incident_annotations ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE incident_node_events ALTER COLUMN id SET DEFAULT uuid_generate_v7();
ALTER TABLE event_catalog_entries ALTER COLUMN id SET DEFAULT uuid_generate_v7();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE alert_rules ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE alerts ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE request_reply_probes ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE incident_annotations ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE incident_node_events ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE event_catalog_entries ALTER COLUMN id SET DEFAULT gen_random_uuid();
DROP FUNCTION IF EXISTS uuid_generate_v7();
-- +goose StatementEnd
