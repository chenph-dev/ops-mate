-- 恢复两层：mysql/postgres 资产回退为 protocol='jdbc' + driver + database
ALTER TABLE hosts ADD COLUMN driver TEXT NOT NULL DEFAULT 'mysql';
ALTER TABLE hosts ADD COLUMN database TEXT NOT NULL DEFAULT '';
UPDATE hosts SET driver = protocol,
    database = json_extract(params_json, '$.database'),
    protocol = 'jdbc'
WHERE protocol IN ('mysql', 'postgres');
ALTER TABLE hosts DROP COLUMN params_json;
