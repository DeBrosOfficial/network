import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import Heading from '@theme/Heading';

import styles from './index.module.css';

function HeroSection() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <section className={styles.hero}>
      <div className={styles.heroBackground}>
        <div className={styles.gridPattern}></div>
        <div className={styles.glowOrb1}></div>
        <div className={styles.glowOrb2}></div>
        <div className={styles.glowOrb3}></div>
      </div>
      
      <div className="container">
        <div className={styles.heroContent}>
          <div className={styles.badge}>
            <span className={styles.badgeText}>🚀 Beta Release</span>
          </div>
          
          <Heading as="h1" className={styles.heroTitle}>
            <span className={styles.titleGradient}>Debros Network</span>
          </Heading>
          
          <p className={styles.heroSubtitle}>
            {siteConfig.tagline}
          </p>
          
          <p className={styles.heroDescription}>
            Build scalable, decentralized applications with ease using our next-generation 
            framework powered by OrbitDB, IPFS, and cutting-edge web technologies.
          </p>
          
          <div className={styles.heroButtons}>
            <Link
              className={clsx('button button--primary button--lg', styles.primaryButton)}
              to="/docs/getting-started">
              Get Started
              <span className={styles.buttonIcon}>→</span>
            </Link>
            
            <Link
              className={clsx('button button--secondary button--lg', styles.secondaryButton)}
              to="/docs/intro">
              View Documentation
            </Link>
          </div>
          
          <div className={styles.quickStart}>
            <span className={styles.quickStartLabel}>Quick Start:</span>
            <code className={styles.codeSnippet}>
              npm create debros-app my-app
            </code>
          </div>
        </div>
      </div>
    </section>
  );
}

function StatsSection() {
  return (
    <section className={styles.stats}>
      <div className="container">
        <div className={styles.statsGrid}>
          <div className={styles.statItem}>
            <div className={styles.statNumber}>100%</div>
            <div className={styles.statLabel}>TypeScript</div>
          </div>
          <div className={styles.statItem}>
            <div className={styles.statNumber}>Zero</div>
            <div className={styles.statLabel}>Config</div>
          </div>
          <div className={styles.statItem}>
            <div className={styles.statNumber}>∞</div>
            <div className={styles.statLabel}>Scalability</div>
          </div>
          <div className={styles.statItem}>
            <div className={styles.statNumber}>🌐</div>
            <div className={styles.statLabel}>Decentralized</div>
          </div>
        </div>
      </div>
    </section>
  );
}

function VideoSection() {
  return (
    <section className={styles.videoSection}>
      <div className="container">
        <div className={styles.videoHeader}>
          <Heading as="h2" className={styles.videoTitle}>
            See Debros Network in Action
          </Heading>
          <p className={styles.videoSubtitle}>
            Watch our comprehensive tutorial to learn how to build your first decentralized application
          </p>
        </div>
        <div className={styles.videoContainer}>
          <div className={styles.videoWrapper}>
            {/* Replace VIDEO_ID_HERE with your actual YouTube video ID */}
            <iframe
              className={styles.videoEmbed}
              src="https://www.youtube.com/embed/VIDEO_ID_HERE?rel=0&modestbranding=1&autohide=1&showinfo=0"
              title="Debros Network Tutorial"
              frameBorder="0"
              allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
              allowFullScreen
            ></iframe>
          </div>
          <div className={styles.videoFeatures}>
            <div className={styles.videoFeature}>
              🚀 <strong>Quick Start:</strong> Get up and running in under 5 minutes
            </div>
            <div className={styles.videoFeature}>
              📚 <strong>Step-by-Step:</strong> Follow along with detailed explanations
            </div>
            <div className={styles.videoFeature}>
              🔧 <strong>Best Practices:</strong> Learn the recommended patterns and approaches
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={siteConfig.title}
      description="Next-Generation Decentralized Web Framework - Build scalable, decentralized applications with ease using Debros Network">
      <HeroSection />
      <StatsSection />
      <main>
        <VideoSection />
        <HomepageFeatures />
      </main>
    </Layout>
  );
}
