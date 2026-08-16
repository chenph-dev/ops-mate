-- 回退：mysql/postgres 资产回退为两层（driver 列 up 未删，无需重建），database 从 params_json 取（空则 ''）
UPDATE hosts SET driver = protocol,
    database = COALESCE(json_extract(params_json, '$.database'), ''),
    protocol = 'jdbc'
WHERE protocol IN ('mysql', 'postgres');
ALTER TABLE hosts DROP COLUMN params_json;
