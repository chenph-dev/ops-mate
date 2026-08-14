-- 连接类型单层化 + driver 专属参数 JSON 化：
-- 存量 jdbc 资产（protocol='jdbc', driver=mysql/postgres）转为
-- protocol=driver 单层标识，database 移入 params_json。
ALTER TABLE hosts ADD COLUMN params_json TEXT NOT NULL DEFAULT '{}';
UPDATE hosts SET protocol = driver, params_json = json_object('database', database) WHERE protocol = 'jdbc';
