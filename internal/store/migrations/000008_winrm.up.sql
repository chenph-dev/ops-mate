-- WinRM/Windows 主机支持：protocol 区分连接协议（ssh/winrm）；rdp_port 用于 Windows 远程桌面。
ALTER TABLE hosts ADD COLUMN protocol TEXT NOT NULL DEFAULT 'ssh';
ALTER TABLE hosts ADD COLUMN rdp_port INTEGER NOT NULL DEFAULT 3389;
