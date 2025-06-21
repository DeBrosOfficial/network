# Blog Scenario - Real Integration Tests

This directory contains comprehensive Docker-based integration tests for the DebrosFramework blog scenario. These tests validate real-world functionality including IPFS private swarm networking, cross-node data synchronization, and complete blog workflow operations.

## Overview

The blog scenario tests a complete blogging platform built on DebrosFramework, including:

- **User Management**: Registration, authentication, profile management
- **Content Creation**: Categories, posts, drafts, publishing
- **Comment System**: Comments, replies, moderation, engagement
- **Cross-Node Sync**: Data consistency across multiple nodes
- **Network Resilience**: Peer connections, private swarm functionality

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Blog Node 1   │    │   Blog Node 2   │    │   Blog Node 3   │
│   Port: 3001    │◄──►│   Port: 3002    │◄──►│   Port: 3003    │
│   IPFS: 4011    │    │   IPFS: 4012    │    │   IPFS: 4013    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │ Bootstrap Node  │
                    │   IPFS: 4001    │
                    │ Private Swarm   │
                    └─────────────────┘
```

## Test Structure

```
blog-scenario/
├── docker/
│   ├── docker-compose.blog.yml     # Docker orchestration
│   ├── Dockerfile.blog-api         # Blog API server image
│   ├── Dockerfile.bootstrap        # IPFS bootstrap node
│   ├── Dockerfile.test-runner      # Test execution environment
│   ├── blog-api-server.ts          # Blog API implementation
│   ├── bootstrap-config.sh         # Bootstrap node configuration
│   └── swarm.key                   # Private IPFS swarm key
├── models/
│   ├── BlogModels.ts               # User, Post, Comment, Category models
│   └── BlogValidation.ts           # Input validation and sanitization
├── scenarios/
│   └── BlogTestRunner.ts           # Test execution utilities
├── tests/
│   └── blog-workflow.test.ts       # Main test suite
├── run-tests.ts                    # Test orchestration script
└── README.md                       # This file
```

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- Node.js 18+ for development
- At least 8GB RAM (recommended for multiple nodes)
- Available ports: 3001-3003, 4001, 4011-4013

### Running Tests

#### Option 1: Full Docker-based Test (Recommended)

```bash
# Run complete integration tests
npm run test:blog-real

# Or use the test runner for better control
npm run test:blog-runner
```

#### Option 2: Build and Run Manually

```bash
# Build Docker images
npm run test:blog-build

# Run tests
npm run test:blog-real

# Clean up afterwards
npm run test:blog-clean
```

#### Option 3: Development Mode

```bash
# Start services only (for debugging)
cd tests/real-integration/blog-scenario
docker-compose -f docker/docker-compose.blog.yml up blog-node-1 blog-node-2 blog-node-3

# Run tests against running services
npm run test:blog-integration
```

## Test Scenarios

### 1. User Management Workflow

- ✅ Cross-node user creation and synchronization
- ✅ User profile updates across nodes
- ✅ User authentication state management

### 2. Category Management

- ✅ Category creation and sync
- ✅ Slug generation and uniqueness
- ✅ Category hierarchy support

### 3. Content Publishing Workflow

- ✅ Draft post creation
- ✅ Post publishing/unpublishing
- ✅ Cross-node content synchronization
- ✅ Post engagement (views, likes)
- ✅ Content relationships (author, category)

### 4. Comment System

- ✅ Distributed comment creation
- ✅ Nested comments (replies)
- ✅ Comment moderation
- ✅ Comment engagement

### 5. Performance & Scalability

- ✅ Concurrent operations across nodes
- ✅ Data consistency under load
- ✅ Network resilience testing

### 6. Network Tests

- ✅ Private IPFS swarm functionality
- ✅ Peer discovery and connections
- ✅ Data replication verification

## API Endpoints

Each blog node exposes a REST API:

### Users

- `POST /api/users` - Create user
- `GET /api/users/:id` - Get user by ID
- `GET /api/users` - List users (with pagination)
- `PUT /api/users/:id` - Update user
- `POST /api/users/:id/login` - Record login

### Categories

- `POST /api/categories` - Create category
- `GET /api/categories` - List categories
- `GET /api/categories/:id` - Get category by ID

### Posts

- `POST /api/posts` - Create post
- `GET /api/posts/:id` - Get post with relationships
- `GET /api/posts` - List posts (with filters)
- `PUT /api/posts/:id` - Update post
- `POST /api/posts/:id/publish` - Publish post
- `POST /api/posts/:id/unpublish` - Unpublish post
- `POST /api/posts/:id/like` - Like post
- `POST /api/posts/:id/view` - Increment views

### Comments

- `POST /api/comments` - Create comment
- `GET /api/posts/:postId/comments` - Get post comments
- `POST /api/comments/:id/approve` - Approve comment
- `POST /api/comments/:id/like` - Like comment

### Metrics

- `GET /health` - Node health status
- `GET /api/metrics/network` - Network metrics
- `GET /api/metrics/data` - Data count metrics
- `GET /api/metrics/framework` - Framework metrics

## Configuration

### Environment Variables

Each node supports these environment variables:

```bash
NODE_ID=blog-node-1           # Unique node identifier
NODE_PORT=3000                # HTTP API port
IPFS_PORT=4001                # IPFS swarm port
BOOTSTRAP_PEER=blog-bootstrap # Bootstrap node hostname
SWARM_KEY_FILE=/data/swarm.key # Private swarm key path
NODE_ENV=test                 # Environment mode
```

### Private IPFS Swarm

The tests use a private IPFS swarm with a shared key to ensure:

- ✅ Network isolation from public IPFS
- ✅ Controlled peer discovery
- ✅ Predictable network topology
- ✅ Enhanced security for testing

## Monitoring and Debugging

### View Logs

```bash
# Follow all container logs
docker-compose -f docker/docker-compose.blog.yml logs -f

# Follow specific service logs
docker-compose -f docker/docker-compose.blog.yml logs -f blog-node-1
```

### Check Node Status

```bash
# Health check
curl http://localhost:3001/health
curl http://localhost:3002/health
curl http://localhost:3003/health

# Network metrics
curl http://localhost:3001/api/metrics/network

# Data metrics
curl http://localhost:3001/api/metrics/data
```

### Connect to Running Containers

```bash
# Access blog node shell
docker-compose -f docker/docker-compose.blog.yml exec blog-node-1 sh

# Check IPFS status
docker-compose -f docker/docker-compose.blog.yml exec blog-bootstrap ipfs swarm peers
```

## Test Data

The tests automatically generate realistic test data:

- **Users**: Various user roles (author, editor, user)
- **Categories**: Technology, Design, Business, etc.
- **Posts**: Different statuses (draft, published, archived)
- **Comments**: Including nested replies and engagement

## Performance Expectations

Based on the test configuration:

- **Node Startup**: < 60 seconds for all nodes
- **Peer Discovery**: < 30 seconds for full mesh
- **Data Sync**: < 15 seconds for typical operations
- **Concurrent Operations**: 20+ simultaneous requests
- **Test Execution**: 5-10 minutes for full suite

## Troubleshooting

### Common Issues

#### Ports Already in Use

```bash
# Check port usage
lsof -i :3001-3003
lsof -i :4001
lsof -i :4011-4013

# Clean up existing containers
npm run test:blog-clean
```

#### Docker Build Failures

```bash
# Clean Docker cache
docker system prune -f

# Rebuild without cache
docker-compose -f docker/docker-compose.blog.yml build --no-cache
```

#### Node Connection Issues

```bash
# Check network connectivity
docker network ls
docker network inspect blog-scenario_blog-network

# Verify swarm key consistency
docker-compose -f docker/docker-compose.blog.yml exec blog-node-1 cat /data/swarm.key
```

#### Test Timeouts

```bash
# Increase test timeout in jest.config.js or test files
# Monitor resource usage
docker stats

# Check available memory and CPU
free -h
```

### Debug Mode

To run tests with additional debugging:

```bash
# Set debug environment
DEBUG=* npm run test:blog-real

# Run with increased verbosity
LOG_LEVEL=debug npm run test:blog-real
```

## Development

### Adding New Tests

1. Add test cases to `tests/blog-workflow.test.ts`
2. Extend `BlogTestRunner` with new utilities
3. Update models if needed in `models/`
4. Test locally before CI integration

### Modifying API

1. Update `blog-api-server.ts`
2. Add corresponding validation in `BlogValidation.ts`
3. Update test scenarios
4. Rebuild Docker images

### Performance Tuning

1. Adjust timeouts in test configuration
2. Modify Docker resource limits
3. Optimize IPFS/OrbitDB configuration
4. Scale node count as needed

## Next Steps

This blog scenario provides a foundation for:

1. **Social Scenario**: User relationships, feeds, messaging
2. **E-commerce Scenario**: Products, orders, payments
3. **Collaborative Scenario**: Real-time editing, conflict resolution
4. **Performance Testing**: Load testing, stress testing
5. **Security Testing**: Attack scenarios, validation testing

The modular design allows easy extension to new scenarios while reusing the infrastructure components.

## Support

For issues or questions:

1. Check the troubleshooting section above
2. Review Docker and test logs
3. Verify your environment meets prerequisites
4. Open an issue with detailed logs and configuration
