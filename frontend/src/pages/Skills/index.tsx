import { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  List,
  Popconfirm,
  Switch,
  message,
  Typography,
} from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import {
  DeleteSkill,
  InstallSkill,
  ListSkills,
  ToggleSkill,
} from '@wailsjs/go/skills/SkillsHandler';

interface SkillInfo {
  name: string;
  title: string;
  description: string;
  enabled: boolean;
}

const { Title, Paragraph } = Typography;

export default function SkillsPage(): React.JSX.Element {
  const { t } = useTranslation('skills');
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async (): Promise<void> => {
    try {
      setSkills(await ListSkills());
    } catch (e) {
      message.error(t('loadFailed', { err: String(e) }));
    }
  }, [t]);

  useEffect(() => {
    ListSkills()
      .then(setSkills)
      .catch(() => {});
  }, []);

  const handleUpload = async (): Promise<void> => {
    setLoading(true);
    try {
      const name = await InstallSkill();
      if (name) {
        message.success(t('uploadSuccess', { name }));
        await load();
      }
    } catch (e) {
      message.error(t('uploadFailed', { err: String(e) }));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: 760, margin: '0 auto' }}>
      <Title level={4}>{t('title')}</Title>
      <Paragraph type="secondary" style={{ fontSize: 12 }}>
        {t('subtitle')}
      </Paragraph>

      <Card size="small">
        <div style={{ marginBottom: 12 }}>
          <Button
            type="primary"
            icon={<UploadOutlined />}
            onClick={() => void handleUpload()}
            loading={loading}
          >
            {t('upload')}
          </Button>
        </div>
        {skills.length === 0 ? (
          <Empty description={t('empty')} />
        ) : (
          <List
            dataSource={skills}
            renderItem={(s) => (
              <List.Item
                actions={[
                  <Switch
                    key="sw"
                    checked={s.enabled}
                    onChange={(v) =>
                      void ToggleSkill(s.name, v)
                        .then(load)
                        .catch((e) =>
                          message.error(t('toggleFailed', { err: String(e) })),
                        )
                    }
                  />,
                  <Popconfirm
                    key="del"
                    title={t('deleteConfirm', { name: s.name })}
                    onConfirm={() =>
                      void DeleteSkill(s.name)
                        .then(load)
                        .catch((e) =>
                          message.error(t('deleteFailed', { err: String(e) })),
                        )
                    }
                  >
                    <Button size="small" danger>
                      {t('delete')}
                    </Button>
                  </Popconfirm>,
                ]}
              >
                <List.Item.Meta
                  title={s.name}
                  description={s.description || s.title}
                />
              </List.Item>
            )}
          />
        )}
      </Card>
    </div>
  );
}
