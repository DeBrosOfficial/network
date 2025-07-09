---
sidebar_position: 2
---

# Development Setup

This guide will help you set up your development environment for contributing to DebrosFramework.

## 🔧 Prerequisites

### Required Software

| Software       | Version | Purpose                     |
| -------------- | ------- | --------------------------- |
| **Node.js**    | 18.0+   | Runtime environment         |
| **pnpm**       | Latest  | Package manager (preferred) |
| **Git**        | Latest  | Version control             |
| **Docker**     | Latest  | Integration testing         |
| **TypeScript** | 5.0+    | Development language        |

### Optional Tools

- **VS Code** - Recommended editor with excellent TypeScript support
- **Docker Desktop** - GUI for Docker management
- **Gitea CLI** (if available) - Command-line interface for our Gitea instance

## 🚀 Environment Setup

### 1. Repository Access

#### Create Gitea Account

1. Visit https://git.debros.io
2. Click "Sign Up" to create an account
3. Verify your email address
4. Request access to the DeBros organization (contact maintainers)

#### Fork and Clone

```bash
# Fork the repository through Gitea web interface
# Then clone your fork
git clone https://git.debros.io/DeBros/network.git
cd network

# Add upstream remote
git remote add upstream https://git.debros.io/DeBros/network.git

# Verify remotes
git remote -v
# origin    https://git.debros.io/DeBros/network.git (fetch)
# origin    https://git.debros.io/DeBros/network.git (push)
# upstream  https://git.debros.io/DeBros/network.git (fetch)
# upstream  https://git.debros.io/DeBros/network.git (push)
```

### 2. Project Setup

#### Install Dependencies

```bash
# Install pnpm if not already installed
npm install -g pnpm

# Install project dependencies
pnpm install

# Install global development tools
pnpm install -g tsx ts-node
```

#### Verify Installation

```bash
# Check versions
node --version    # Should be 18.0+
pnpm --version    # Should be latest
tsc --version     # Should be 5.0+

# Check project setup
pnpm run build   # Should complete without errors
```

### 3. IDE Configuration

#### VS Code Setup

Install recommended extensions:

```bash
# Install VS Code extensions
code --install-extension ms-vscode.vscode-typescript-next
code --install-extension esbenp.prettier-vscode
code --install-extension ms-vscode.vscode-eslint
code --install-extension bradlc.vscode-tailwindcss
code --install-extension ms-vscode.vscode-json
```

Create `.vscode/settings.json`:

```json
{
  "typescript.preferences.includePackageJsonAutoImports": "auto",
  "typescript.suggest.autoImports": true,
  "editor.formatOnSave": true,
  "editor.defaultFormatter": "esbenp.prettier-vscode",
  "editor.codeActionsOnSave": {
    "source.fixAll.eslint": true
  },
  "typescript.preferences.importModuleSpecifier": "relative",
  "files.exclude": {
    "**/node_modules": true,
    "**/dist": true,
    "**/.git": true
  }
}
```

#### TypeScript Configuration

The project includes proper TypeScript configuration in `tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "node",
    "experimentalDecorators": true,
    "emitDecoratorMetadata": true,
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "declaration": true,
    "outDir": "./dist",
    "rootDir": "./src"
  }
}
```

## 🧪 Development Workflow

### Running the Framework

#### Basic Build and Test

```bash
# Clean build
pnpm run clean
pnpm run build

# Run unit tests
pnpm run test:unit

# Run integration tests (requires Docker)
pnpm run test:real

# Run specific blog integration test
pnpm run test:blog-integration
```

#### Development Commands

```bash
# Watch mode for development
pnpm run dev

# Linting and formatting
pnpm run lint          # Check for lint errors
pnpm run lint:fix      # Fix auto-fixable lint errors
pnpm run format        # Format code with Prettier

# Clean up
pnpm run clean         # Remove build artifacts
```

### Testing Setup

#### Unit Tests

Unit tests use Jest and are located in `tests/unit/`:

```bash
# Run all unit tests
pnpm run test:unit

# Run specific test file
npx jest tests/unit/framework/core/ConfigManager.test.ts

# Run tests in watch mode
npx jest --watch tests/unit/
```

#### Integration Tests

Integration tests use Docker to create real IPFS/OrbitDB environments:

```bash
# Ensure Docker is running
docker --version

# Run full integration test suite
pnpm run test:real

# This will:
# 1. Build Docker containers
# 2. Start IPFS bootstrap node
# 3. Start multiple framework instances
# 4. Run real-world scenarios
# 5. Tear down containers
```

#### Test Structure

```
tests/
├── unit/                    # Unit tests
│   ├── framework/          # Framework component tests
│   └── shared/             # Shared test utilities
└── real-integration/       # Integration tests
    └── blog-scenario/      # Blog application test scenario
        ├── docker/         # Docker configuration
        ├── scenarios/      # Test scenarios
        └── models/         # Test models
```

### Docker Development

#### Docker Setup for Testing

The integration tests require Docker. Make sure you have:

1. **Docker Desktop** installed and running
2. **Docker Compose** available (included with Docker Desktop)
3. **Sufficient memory** allocated to Docker (4GB+ recommended)

#### Docker Commands

```bash
# Build test containers
docker-compose -f tests/real-integration/blog-scenario/docker/docker-compose.blog.yml build

# Run integration tests
pnpm run test:real

# Clean up Docker resources
docker system prune -f
```

## 🔄 Git Workflow

### Branch Strategy

We use a feature branch workflow:

```bash
# Start from main branch
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/your-feature-name

# Make changes, commit, and push
git add .
git commit -m "feat: add new feature"
git push origin feature/your-feature-name
```

### Pull Request Process

1. **Create Pull Request** in Gitea web interface
2. **Fill out template** with description and testing notes
3. **Request review** from maintainers
4. **Address feedback** if any
5. **Merge** once approved

## 🛠️ Development Tools

### Code Quality Tools

#### ESLint Configuration

The project uses ESLint for code quality:

```bash
# Check for issues
pnpm run lint

# Fix auto-fixable issues
pnpm run lint:fix
```

#### Prettier Configuration

Prettier handles code formatting:

```bash
# Format all files
pnpm run format

# Format specific files
npx prettier --write "src/**/*.ts"
```

#### Husky Git Hooks

The project includes pre-commit hooks:

- **Pre-commit**: Runs lint-staged to format staged files
- **Pre-push**: Runs basic tests to prevent broken code

### Package Scripts Reference

```json
{
  "scripts": {
    "build": "tsc && tsc-esm-fix --outDir=./dist/esm",
    "dev": "tsc -w",
    "clean": "rimraf dist",
    "lint": "npx eslint src",
    "format": "prettier --write \"**/*.{ts,js,json,md}\"",
    "lint:fix": "npx eslint src --fix",
    "test:unit": "jest tests/unit",
    "test:blog-integration": "tsx tests/real-integration/blog-scenario/scenarios/BlogTestRunner.ts",
    "test:real": "docker-compose -f tests/real-integration/blog-scenario/docker/docker-compose.blog.yml up --build --abort-on-container-exit",
    "prepublishOnly": "npm run clean && npm run build"
  }
}
```

## 🔍 Debugging

### Framework Debugging

#### Enable Debug Logging

```typescript
// In your test or development code
const framework = new DebrosFramework({
  monitoring: {
    logLevel: 'debug',
    enableMetrics: true,
  },
});
```

#### VS Code Debugging

Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Unit Tests",
      "type": "node",
      "request": "launch",
      "program": "${workspaceFolder}/node_modules/.bin/jest",
      "args": ["--runInBand", "${file}"],
      "console": "integratedTerminal",
      "internalConsoleOptions": "neverOpen",
      "env": {
        "NODE_ENV": "test"
      }
    },
    {
      "name": "Debug Integration Test",
      "type": "node",
      "request": "launch",
      "program": "${workspaceFolder}/node_modules/.bin/tsx",
      "args": ["tests/real-integration/blog-scenario/scenarios/BlogTestRunner.ts"],
      "console": "integratedTerminal",
      "env": {
        "NODE_ENV": "test"
      }
    }
  ]
}
```

### Common Issues

#### Node.js Version Issues

```bash
# Check Node.js version
node --version

# If using nvm, switch to correct version
nvm use 18
nvm install 18.19.0
nvm alias default 18.19.0
```

#### Docker Issues

```bash
# Check Docker status
docker --version
docker-compose --version

# Clean Docker cache if tests fail
docker system prune -f
docker volume prune -f
```

#### TypeScript Issues

```bash
# Clear TypeScript cache
npx tsc --build --clean

# Rebuild project
pnpm run clean
pnpm run build
```

## 📚 Additional Resources

### Learning Resources

- **[IPFS Documentation](https://docs.ipfs.io/)** - Understanding IPFS concepts
- **[OrbitDB Guide](https://orbitdb.org/getting-started/)** - OrbitDB basics
- **[libp2p Concepts](https://docs.libp2p.io/concepts/)** - Peer-to-peer networking
- **[TypeScript Handbook](https://www.typescriptlang.org/docs/)** - TypeScript reference

### Community Resources

- **Gitea Repository**: https://git.debros.io/DeBros/network
- **Documentation**: This documentation site
- **Discord** (if available): Community chat
- **Email**: Contact maintainers for access and questions

---

## ✅ Setup Verification

Run this checklist to verify your setup:

```bash
# 1. Check Node.js version
node --version              # Should be 18.0+

# 2. Check package manager
pnpm --version              # Should be latest

# 3. Install dependencies
pnpm install                # Should complete without errors

# 4. Build project
pnpm run build              # Should complete without errors

# 5. Run linting
pnpm run lint               # Should pass with no errors

# 6. Run unit tests
pnpm run test:unit          # Should pass all tests

# 7. Check Docker (optional, for integration tests)
docker --version           # Should show Docker version
pnpm run test:real          # Should run integration tests
```

If all steps pass, you're ready to contribute! 🎉

---

**Next Steps:**

- Read our **[Code Guidelines](./code-guidelines)** to understand coding standards
- Check out **[Testing Guide](./testing-guide)** to learn about writing tests
- Browse existing issues in Gitea to find something to work on
