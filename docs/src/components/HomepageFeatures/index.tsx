import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import styles from './styles.module.css';

type FeatureItem = {
  title: string;
  icon: string;
  description: ReactNode;
  highlight?: boolean;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Decentralized by Design',
    icon: '🌐',
    description: (
      <>
        Built on <strong>OrbitDB</strong> and <strong>IPFS</strong>, DebrosFramework 
        creates truly decentralized applications that don't rely on centralized servers 
        or single points of failure.
      </>
    ),
    highlight: true,
  },
  {
    title: 'Developer Experience First',
    icon: '⚡',
    description: (
      <>
        Full <strong>TypeScript</strong> support with intelligent auto-completion, 
        decorator-based models, and intuitive APIs that make building complex 
        applications feel effortless.
      </>
    ),
  },
  {
    title: 'Infinite Scalability',
    icon: '🚀',
    description: (
      <>
        Automatic sharding, efficient queries, and built-in caching ensure your 
        applications can scale to millions of users without architectural changes.
      </>
    ),
  },
  {
    title: 'Zero Configuration',
    icon: '🎯',
    description: (
      <>
        Start building immediately with sensible defaults. No complex setup, 
        no configuration files, no DevOps headaches—just pure development focus.
      </>
    ),
  },
  {
    title: 'Real-time Sync',
    icon: '🔄',
    description: (
      <>
        Built-in real-time synchronization across all peers. Changes propagate 
        instantly across the network with conflict resolution and offline support.
      </>
    ),
  },
  {
    title: 'Enterprise Ready',
    icon: '🔒',
    description: (
      <>
        Production-grade security, comprehensive testing, detailed documentation, 
        and enterprise support make DebrosFramework ready for mission-critical applications.
      </>
    ),
  },
];

function Feature({title, icon, description, highlight}: FeatureItem) {
  return (
    <div className={clsx(styles.featureCard, highlight && styles.featureCardHighlight)}>
      <div className={styles.featureIcon}>
        <span className={styles.iconEmoji}>{icon}</span>
      </div>
      <div className={styles.featureContent}>
        <Heading as="h3" className={styles.featureTitle}>{title}</Heading>
        <p className={styles.featureDescription}>{description}</p>
      </div>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className={styles.featuresHeader}>
          <Heading as="h2" className={styles.featuresTitle}>
            Why Choose Debros Network?
          </Heading>
          <p className={styles.featuresSubtitle}>
            Everything you need to build the next generation of decentralized applications
          </p>
        </div>
        <div className={styles.featuresGrid}>
          {FeatureList.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
