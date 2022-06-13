import { getDatasourceSrv } from 'app/features/plugins/datasource_srv';
import { getBackendSrv } from 'app/core/services/backend_srv';
import { SetupStep } from './types';

export const getSteps = (): SetupStep[] => [
  {
    heading: 'Welcome to SolidInsight',
    subheading: 'The steps below will guide you to quickly finish setting up your Organisation.',
    title: 'Basic',
    info: 'The steps below will guide you to quickly finish setting up your SolidInsight Platform.',
    done: false,
    cards: [
      {
        type: 'docs',
        title: 'Add your first data source',
        heading: 'data sources',
        icon: 'database',
        href: 'datasources/new',
        check: () => {
          return new Promise((resolve) => {
            resolve(
              getDatasourceSrv()
                .getMetricSources()
                .filter((item) => {
                  return item.meta.builtIn !== true;
                }).length > 0
            );
          });
        },
        done: false,
      },
      {
        type: 'docs',
        heading: 'dashboards',
        title: 'Create your first dashboard',
        icon: 'apps',
        href: 'dashboard/new',
        check: async () => {
          const result = await getBackendSrv().search({ limit: 1 });
          return result.length > 0;
        },
        done: false,
      },
    ],
  },
];
