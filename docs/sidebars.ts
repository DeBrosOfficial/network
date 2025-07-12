import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */
const sidebars: SidebarsConfig = {
  // Main documentation sidebar
  tutorialSidebar: [
    'intro',
    'getting-started',
    {
      type: 'category',
      label: 'Core Concepts',
      items: [
        'core-concepts/architecture',
        'core-concepts/models',
        'core-concepts/decorators',
        'core-concepts/database-management',
      ],
    },
    {
      type: 'category',
      label: 'Query System',
      items: [
        'query-system/query-builder',
        'query-system/relationships',
      ],
    },
    {
      type: 'category',
      label: 'Advanced Topics',
      items: [
        'advanced/migrations',
        'advanced/performance',
        'advanced/automatic-pinning',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/migration-guide',
      ],
    },
    {
      type: 'category',
      label: 'Examples',
      items: [
        'examples/basic-usage',
        'examples/complex-queries',
        'examples/migrations',
        'examples/social-platform',
        'examples/working-examples',
      ],
    },
    {
      type: 'category',
      label: 'Internals',
      items: [
        'internals/behind-the-scenes',
      ],
    },
    {
      type: 'category',
      label: 'Video Tutorials',
      items: [
        'videos/index',
      ],
    },
    {
      type: 'category',
      label: 'Contributing',
      items: [
        'contributing/overview',
        'contributing/development-setup',
        'contributing/code-guidelines',
        'contributing/community',
        'contributing/documentation-guide',
        'contributing/testing-guide',
        'contributing/release-process',
      ],
    },
  ],

  // API Reference sidebar
  apiSidebar: [
    'api/overview',
    {
      type: 'category',
      label: 'Core Classes',
      items: [
        'api/debros-framework',
        'api/base-model',
        'api/query-builder',
        'api/query-executor',
        'api/database-manager',
        'api/shard-manager',
        'api/relationship-manager',
      ],
    },
    {
      type: 'category',
      label: 'Migration System',
      items: [
        'api/migration-builder',
        'api/migration-manager',
      ],
    },
    {
      type: 'category',
      label: 'Decorators',
      items: [
        'api/decorators/model',
        'api/decorators/field',
        'api/decorators/relationships',
        'api/decorators/hooks',
      ],
    },
    {
      type: 'category',
      label: 'Network API',
      items: [
        'api/network-api',
      ],
    },
  ],
};

export default sidebars;
