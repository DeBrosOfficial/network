---
sidebar_position: 6
---

# Release Process

This document outlines the release process for DebrosFramework.

## Overview

DebrosFramework follows semantic versioning and has a structured release process to ensure quality and reliability.

## Version Numbers

We use semantic versioning (SemVer):
- **Major** (X.0.0) - Breaking changes
- **Minor** (X.Y.0) - New features, backwards compatible
- **Patch** (X.Y.Z) - Bug fixes, backwards compatible

## Release Types

### Regular Releases

Regular releases happen monthly and include:
- New features
- Bug fixes
- Performance improvements
- Documentation updates

### Hotfix Releases

Hotfix releases address critical issues:
- Security vulnerabilities
- Major bugs affecting production
- Data integrity issues

## Release Process

### 1. Preparation

- Ensure all tests pass
- Update documentation
- Review breaking changes

### 2. Version Bump

- Update version in package.json
- Update CHANGELOG.md
- Create git tag

### 3. Deployment

- Build and test
- Publish to npm
- Deploy documentation

## Quality Assurance

- All releases must pass CI/CD
- Manual testing on critical paths
- Community feedback integration

## Related Documents

- [Code Guidelines](./code-guidelines) - Coding standards
- [Testing Guide](./testing-guide) - Testing practices
