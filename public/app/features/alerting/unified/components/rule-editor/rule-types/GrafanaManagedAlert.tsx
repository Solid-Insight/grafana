import React, { FC } from 'react';
import { RuleType, SharedProps } from './RuleType';
import { RuleFormType } from '../../../types/rule-form';

const GrafanaManagedRuleType: FC<SharedProps> = ({ selected = false, disabled, onClick }) => {
  return (
    <RuleType
      name="SolidInsight managed alert"
      description={
        <span>
          Supports multiple data sources of any kind.
          <br />
          Transform data with expressions.
        </span>
      }
      image="/public/img/solidinsight_icon.svg"
      selected={selected}
      disabled={disabled}
      value={RuleFormType.grafana}
      onClick={onClick}
    />
  );
};

export { GrafanaManagedRuleType };
