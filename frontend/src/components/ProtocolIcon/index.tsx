import { useMemo } from 'react';

// 静态导入所有图标（Vite 会处理为 URL）
import mysql from '@/assets/icons/mysql.png';
import sqlite from '@/assets/icons/sqlite.png';
import postgres from '@/assets/icons/postgres.png';
import redis from '@/assets/icons/redis.png';
import mongodb from '@/assets/icons/mongodb.png';
import clickhouse from '@/assets/icons/clickhouse.png';
import elasticsearch from '@/assets/icons/elasticsearch.png';
import doris from '@/assets/icons/doris.svg';
import tidb from '@/assets/icons/tidb.png';
import oracle from '@/assets/icons/oracle.png';
import sqlserver from '@/assets/icons/sqlserver.png';
import linux from '@/assets/icons/linux.png';
import windows from '@/assets/icons/windows.png';
import kafka from '@/assets/icons/kafka.png';
import nginx from '@/assets/icons/nginx.png';
import tomcat from '@/assets/icons/tomcat.png';
import rabbitmq from '@/assets/icons/rabbitmq.png';
import zookeeper from '@/assets/icons/zookeeper.png';
import kubernetes from '@/assets/icons/kubernetes.png';
import docker from '@/assets/icons/docker.png';
import prometheus from '@/assets/icons/prometheus.png';

/** protocol → 图标资源映射。新连接器接入时在此添加对应图标即可。 */
const PROTOCOL_ICONS: Record<string, string> = {
  mysql,
  postgres,
  sqlite,
  redis,
  mongodb,
  clickhouse,
  elasticsearch,
  doris,
  tidb,
  oracle,
  sqlserver,
  kafka,
  nginx,
  tomcat,
  rabbitmq,
  zookeeper,
  kubernetes,
  docker,
  prometheus,
};

// 命令型协议图标
const COMMAND_ICONS: Record<string, string> = {
  ssh: linux,
  winrm: windows,
};

const DEFAULT_ICON = linux;

export interface ProtocolIconProps {
  protocol?: string;
  size?: number;
  style?: React.CSSProperties;
}

/** 协议图标组件：按 protocol 匹配产品图标，未匹配则按 kind 回退，最后通用图标。 */
export default function ProtocolIcon({
  protocol = '',
  size = 16,
  style,
}: ProtocolIconProps): React.JSX.Element {
  const icon = useMemo(() => {
    const key = protocol.toLowerCase();
    return PROTOCOL_ICONS[key] ?? COMMAND_ICONS[key] ?? DEFAULT_ICON;
  }, [protocol]);

  return (
    <img
      src={icon}
      alt={protocol}
      width={size}
      height={size}
      style={{ verticalAlign: 'middle', ...style }}
    />
  );
}
