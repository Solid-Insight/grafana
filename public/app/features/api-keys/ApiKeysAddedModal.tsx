﻿import React, { useCallback } from 'react';
import { css } from '@emotion/css';
import { Alert, Field, Modal, useStyles2, Input, Icon, ClipboardButton } from '@grafana/ui';
import { GrafanaTheme2 } from '@grafana/data';
import { notifyApp } from '../../core/actions';
import { dispatch } from '../../store/store';
import { createSuccessNotification } from '../../core/copy/appNotification';

export interface Props {
  onDismiss: () => void;
  apiKey: string;
  rootPath: string;
}

export function ApiKeysAddedModal({ onDismiss, apiKey, rootPath }: Props): JSX.Element {
  const styles = useStyles2(getStyles);
  const getClipboardText = useCallback(() => apiKey, [apiKey]);
  const onClipboardCopy = () => {
    dispatch(notifyApp(createSuccessNotification('Content copied to clipboard')));
  };
  return (
    <Modal title="API Key Created" onDismiss={onDismiss} onClickBackdrop={onDismiss} isOpen>
      <Field label="Key">
        <Input
          id="Key"
          value={apiKey}
          readOnly
          addonAfter={
            <ClipboardButton variant="primary" getText={getClipboardText} onClipboardCopy={onClipboardCopy}>
              <Icon name="copy" /> Copy
            </ClipboardButton>
          }
        />
      </Field>
      <Alert severity="info" title="You will only be able to view this key here once!">
        It is not stored in this form, so be sure to copy it now.
      </Alert>

      <p className="text-muted">You can authenticate a request using the Authorization HTTP header, example:</p>
      <pre className={styles.small}>
        curl -H &quot;Authorization: Bearer {apiKey}&quot; {rootPath}/api/dashboards/home
      </pre>
    </Modal>
  );
}

function getStyles(theme: GrafanaTheme2) {
  return {
    label: css`
      padding: ${theme.spacing(1)};
      background-color: ${theme.colors.background.secondary};
      border-radius: ${theme.shape.borderRadius()};
    `,
    small: css`
      font-size: ${theme.typography.bodySmall.fontSize};
      font-weight: ${theme.typography.bodySmall.fontWeight};
    `,
  };
}
