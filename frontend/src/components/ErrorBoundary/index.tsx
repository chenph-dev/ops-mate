import { Component, type ReactNode } from 'react';
import { Button, Result } from 'antd';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

/** 错误边界：捕获子树渲染异常，展示恢复页面而非白屏崩溃。 */
export default class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    // 开发期输出到控制台，便于排查
    console.error('[ErrorBoundary]', error, info.componentStack);
  }

  handleReset = (): void => {
    this.setState({ hasError: false, error: null });
  };

  render(): ReactNode {
    if (this.state.hasError) {
      return <ErrorFallback onReset={this.handleReset} />;
    }
    return this.props.children;
  }
}

function ErrorFallback({ onReset }: { onReset: () => void }): React.JSX.Element {
  const { t } = useTranslation('common');
  const navigate = useNavigate();

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', padding: 24 }}>
      <Result
        status="error"
        title={t('errorTitle')}
        subTitle={t('errorSubTitle')}
        extra={[
          <Button key="home" onClick={() => navigate('/', { replace: true })}>
            {t('errorBackHome')}
          </Button>,
          <Button key="retry" type="primary" onClick={onReset}>
            {t('errorRetry')}
          </Button>,
        ]}
      />
    </div>
  );
}
