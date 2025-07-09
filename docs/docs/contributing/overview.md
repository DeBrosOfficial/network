---
sidebar_position: 1
---

# Contributing to DebrosFramework

Welcome to the DebrosFramework contributor community! We're excited to have you help build the future of decentralized application development.

## 🌟 Why Contribute?

DebrosFramework is an ambitious project that aims to make decentralized application development as simple as traditional web development. Your contributions help:

- **Advance decentralized technology** - Make dApps more accessible to developers
- **Build better tools** - Create powerful abstractions over IPFS and OrbitDB
- **Shape the future** - Influence how decentralized applications are built
- **Learn cutting-edge tech** - Work with the latest in distributed systems
- **Join a community** - Connect with like-minded developers

## 🚀 Development Status

**Current Version**: 0.5.0-beta  
**Status**: Active Development - Beta Release

DebrosFramework is in active beta development. The core architecture is not stable, but we're continuously improving APIs, adding features, and optimizing performance. This is an excellent time to contribute as your input can significantly shape the framework's direction.

### What's Ready for Contribution

✅ **Core Framework** - Stable architecture, ready for enhancements  
✅ **Model System** - Decorator-based models with validation  
✅ **Query Builder** - Rich querying with optimization  
✅ **Relationship System** - Complex data relationships  
✅ **Sharding** - Automatic data distribution  
✅ **Migration System** - Schema evolution tools  
✅ **Documentation** - Comprehensive guides and examples

### What's Coming Next

🔄 **Performance Optimization** - Query caching and execution improvements  
🔄 **Advanced Features** - Real-time subscriptions and event systems  
🔄 **Developer Experience** - Better tooling and debugging  
🔄 **Production Ready** - Stability and performance for production use

## 🏗️ Repository Information

### Self-Hosted Git Repository

We use a **self-hosted Gitea instance** instead of GitHub:

**Repository URL**: https://git.debros.io/DeBros/network

**Why Gitea?**

- **Decentralization aligned** - Fits our philosophy of decentralized systems
- **Full control** - Complete control over our development infrastructure
- **Privacy focused** - No external dependencies for sensitive development data
- **Community owned** - Aligns with our open-source, community-driven approach

### Getting Repository Access

1. **Create an account** at https://git.debros.io
2. **Request access** by contacting the maintainers
3. **Fork the repository** to your account
4. **Clone your fork** locally

```bash
git clone https://git.debros.io/DeBros/network.git
cd network
```

## 🤝 How to Contribute

### Types of Contributions Welcome

| Type                    | Description                    | Skill Level           | Time Commitment |
| ----------------------- | ------------------------------ | --------------------- | --------------- |
| 🐛 **Bug Reports**      | Find and report issues         | Beginner              | Low             |
| 📖 **Documentation**    | Improve guides and examples    | Beginner-Intermediate | Low-Medium      |
| ✨ **Feature Requests** | Suggest new capabilities       | Any                   | Low             |
| 🧪 **Testing**          | Write tests and test scenarios | Intermediate          | Medium          |
| 🔧 **Bug Fixes**        | Fix reported issues            | Intermediate          | Medium          |
| ⚡ **Performance**      | Optimize existing code         | Advanced              | Medium-High     |
| 🚀 **New Features**     | Implement new functionality    | Advanced              | High            |
| 🏗️ **Architecture**     | Design system improvements     | Expert                | High            |

### Contribution Areas

#### 🎯 High Priority Areas

1. **Integration Tests** - Real-world scenario testing
2. **Performance Optimization** - Query execution and caching
3. **Developer Experience** - Better error messages and debugging
4. **Documentation** - More examples and use cases
5. **Type Safety** - Improved TypeScript definitions

#### 🔥 Hot Topics

- **Real-time Features** - PubSub improvements and real-time data sync
- **Migration Tools** - Better schema evolution and data transformation
- **Query Optimization** - Smarter query planning and execution
- **Monitoring** - Performance metrics and health checking
- **CLI Tools** - Development and deployment tooling

## 🛠️ Technical Overview

### Architecture Understanding

Before contributing code, familiarize yourself with the framework architecture:

```
DebrosFramework/
├── Core Layer (ConfigManager, DatabaseManager)
├── Model Layer (BaseModel, Decorators, Validation)
├── Query Layer (QueryBuilder, QueryExecutor, Optimization)
├── Relationship Layer (RelationshipManager, LazyLoader)
├── Sharding Layer (ShardManager, Distribution)
├── Migration Layer (MigrationManager, MigrationBuilder)
├── Feature Layer (PinningManager, PubSubManager)
└── Service Layer (OrbitDBService, IPFSService)
```

### Key Technologies

- **TypeScript** - Primary development language
- **OrbitDB** - Distributed database layer
- **IPFS/Helia** - Distributed storage layer
- **libp2p** - Peer-to-peer networking
- **Docker** - Containerization for testing
- **Jest** - Unit testing framework
- **Prettier/ESLint** - Code formatting and linting

### Development Philosophy

1. **Developer Experience First** - APIs should be intuitive and well-documented
2. **Type Safety** - Comprehensive TypeScript support throughout
3. **Performance by Default** - Optimize common use cases automatically
4. **Flexibility** - Support diverse application patterns
5. **Reliability** - Robust error handling and recovery
6. **Scalability** - Design for applications with millions of users

## 📋 Getting Started Checklist

Before making your first contribution:

- [ ] Read this contributor guide completely
- [ ] Set up your development environment
- [ ] Run the test suite successfully
- [ ] Explore the codebase and documentation
- [ ] Join our community channels
- [ ] Choose your first contribution area
- [ ] Check existing issues and discussions

## 🔗 Quick Links

- **[Development Setup](./development-setup)** - Get your environment ready
- **[Code Guidelines](./code-guidelines)** - Coding standards and best practices
- **[Testing Guide](./testing-guide)** - How to write and run tests
- **[Documentation Guide](./documentation-guide)** - Contributing to docs
- **[Release Process](./release-process)** - How we ship new versions
- **[Community](./community)** - Connect with other contributors

## 💡 First Time Contributors

New to open source or DebrosFramework? Start here:

1. **Good First Issues** - Look for issues tagged `good-first-issue` in our Gitea repository
2. **Documentation** - Help improve our guides and examples
3. **Testing** - Add test cases for existing functionality
4. **Examples** - Create new usage examples and tutorials

## 🎯 Contributor Levels

### 🌱 **Beginner Contributors**

- Report bugs and suggest improvements
- Fix typos and improve documentation
- Add simple test cases
- Create usage examples

### 🌿 **Intermediate Contributors**

- Fix bugs and implement small features
- Improve existing functionality
- Write comprehensive tests
- Contribute to API design discussions

### 🌳 **Advanced Contributors**

- Implement major features
- Optimize performance-critical code
- Design architectural improvements
- Mentor other contributors

### 🏆 **Core Contributors**

- Drive technical direction
- Review and merge contributions
- Manage releases and roadmap
- Represent the project publicly

## 🏅 Recognition

Contributors are recognized through:

- **Contributor list** in repository and documentation
- **Release notes** crediting significant contributions
- **Community highlights** in announcements
- **Direct contributor access** for consistent contributors
- **Maintainer status** for exceptional long-term contributors

---

Ready to contribute? Head over to our **[Development Setup Guide](./development-setup)** to get started!

_Have questions? Join our community channels or reach out to the maintainers. We're here to help! 🚀_
